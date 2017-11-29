package controller

import (
	"fmt"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	kutilapps "github.com/appscode/kutil/apps/v1beta1"
	kutilcore "github.com/appscode/kutil/core/v1"
	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/k8sdb/apimachinery/pkg/docker"
	"github.com/k8sdb/apimachinery/pkg/eventer"
	apps "k8s.io/api/apps/v1beta1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) ensureStatefulSet(
	elasticsearch *tapi.Elasticsearch,
	statefulsetName string,
	labels map[string]string,
	replicas int32,
	envList []core.EnvVar,
	isClient bool,
) error {

	if err := c.checkStatefulSet(elasticsearch, statefulsetName); err != nil {
		return err
	}

	statefulsetMeta := metav1.ObjectMeta{
		Name:      statefulsetName,
		Namespace: elasticsearch.Namespace,
	}

	if replicas < 0 {
		replicas = 0
	}

	statefulset, err := kutilapps.CreateOrPatchStatefulSet(c.Client, statefulsetMeta, func(in *apps.StatefulSet) *apps.StatefulSet {
		in = upsertObjectMeta(in, labels, elasticsearch.StatefulSetAnnotations())

		in.Spec.Replicas = types.Int32P(replicas)
		in.Spec.ServiceName = c.opt.GoverningService
		in.Spec.Template = core.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: in.ObjectMeta.Labels,
			},
		}

		in = upsertInitContainer(in)
		in = upsertContainer(in, elasticsearch)
		in = upsertEnv(in, elasticsearch, envList)
		in = upsertPort(in, isClient)

		in.Spec.Template.Spec.NodeSelector = elasticsearch.Spec.NodeSelector
		in.Spec.Template.Spec.Affinity = elasticsearch.Spec.Affinity
		in.Spec.Template.Spec.SchedulerName = elasticsearch.Spec.SchedulerName
		in.Spec.Template.Spec.Tolerations = elasticsearch.Spec.Tolerations

		if isClient {
			in = upsertMonitoringContainer(in, elasticsearch, c.opt.ExporterTag)
			in = upsertDatabaseSecret(in, elasticsearch.Spec.DatabaseSecret.SecretName)
		}

		in = upsertCertificate(in, elasticsearch.Spec.CertificateSecret.SecretName, isClient)
		in = upsertDataVolume(in, elasticsearch)

		if c.opt.EnableRbac {
			in.Spec.Template.Spec.ServiceAccountName = elasticsearch.Name
		}

		in.Spec.UpdateStrategy.Type = apps.RollingUpdateStatefulSetStrategyType

		return in
	})

	if err != nil {
		return err
	}

	if replicas > 0 {
		// Check StatefulSet Pod status
		if err := c.CheckStatefulSetPodStatus(statefulset, durationCheckStatefulSet); err != nil {
			c.recorder.Eventf(
				elasticsearch.ObjectReference(),
				core.EventTypeWarning,
				eventer.EventReasonFailedToStart,
				"Failed to create StatefulSet. Reason: %v",
				err,
			)

			return err
		} else {
			c.recorder.Event(
				elasticsearch.ObjectReference(),
				core.EventTypeNormal,
				eventer.EventReasonSuccessfulCreate,
				"Successfully created StatefulSet",
			)
		}
	}

	return nil
}

func (c *Controller) ensureClientNode(elasticsearch *tapi.Elasticsearch) error {
	statefulsetName := elasticsearch.OffshootName()
	clientNode := elasticsearch.Spec.Topology.Client

	if clientNode.Prefix != "" {
		statefulsetName = fmt.Sprintf("%v-%v", clientNode.Prefix, statefulsetName)
	}

	labels := elasticsearch.StatefulSetLabels()
	labels[NodeRoleClient] = "set"

	envList := []core.EnvVar{
		{
			Name:  "NODE_MASTER",
			Value: fmt.Sprintf("%v", false),
		},
		{
			Name:  "NODE_DATA",
			Value: fmt.Sprintf("%v", false),
		},
		{
			Name:  "MODE",
			Value: "client",
		},
	}

	return c.ensureStatefulSet(elasticsearch, statefulsetName, labels, clientNode.Replicas, envList, true)
}

func (c *Controller) ensureMasterNode(elasticsearch *tapi.Elasticsearch) error {
	statefulsetName := elasticsearch.OffshootName()
	masterNode := elasticsearch.Spec.Topology.Master

	if masterNode.Prefix != "" {
		statefulsetName = fmt.Sprintf("%v-%v", masterNode.Prefix, statefulsetName)
	}

	labels := elasticsearch.StatefulSetLabels()
	labels[NodeRoleMaster] = "set"

	replicas := masterNode.Replicas
	if replicas < 0 {
		replicas = 0
	}

	envList := []core.EnvVar{
		{
			Name:  "NODE_DATA",
			Value: fmt.Sprintf("%v", false),
		},
		{
			Name:  "NODE_INGEST",
			Value: fmt.Sprintf("%v", false),
		},
		{
			Name:  "HTTP_ENABLE",
			Value: fmt.Sprintf("%v", false),
		},
		{
			Name:  "NUMBER_OF_MASTERS",
			Value: fmt.Sprintf("%v", (replicas/2)+1),
		},
	}

	return c.ensureStatefulSet(elasticsearch, statefulsetName, labels, masterNode.Replicas, envList, false)
}

func (c *Controller) ensureDataNode(elasticsearch *tapi.Elasticsearch) error {
	statefulsetName := elasticsearch.OffshootName()
	dataNode := elasticsearch.Spec.Topology.Data

	if dataNode.Prefix != "" {
		statefulsetName = fmt.Sprintf("%v-%v", dataNode.Prefix, statefulsetName)
	}

	labels := elasticsearch.StatefulSetLabels()
	labels[NodeRoleData] = "set"

	envList := []core.EnvVar{
		{
			Name:  "NODE_MASTER",
			Value: fmt.Sprintf("%v", false),
		},
		{
			Name:  "NODE_INGEST",
			Value: fmt.Sprintf("%v", false),
		},
		{
			Name:  "HTTP_ENABLE",
			Value: fmt.Sprintf("%v", false),
		},
	}

	return c.ensureStatefulSet(elasticsearch, statefulsetName, labels, dataNode.Replicas, envList, false)
}

func (c *Controller) ensureCombinedNode(elasticsearch *tapi.Elasticsearch) error {
	statefulsetName := elasticsearch.OffshootName()
	labels := elasticsearch.StatefulSetLabels()
	labels[NodeRoleClient] = "set"
	labels[NodeRoleMaster] = "set"
	labels[NodeRoleData] = "set"

	replicas := elasticsearch.Spec.Replicas
	if replicas < 0 {
		replicas = 0
	}

	envList := []core.EnvVar{
		{
			Name:  "NUMBER_OF_MASTERS",
			Value: fmt.Sprintf("%v", (replicas/2)+1),
		},
		{
			Name:  "MODE",
			Value: "client",
		},
	}

	return c.ensureStatefulSet(elasticsearch, statefulsetName, labels, replicas, envList, true)
}

func (c *Controller) checkStatefulSet(elasticsearch *tapi.Elasticsearch, name string) error {
	elasticsearchName := elasticsearch.OffshootName()
	// SatatefulSet for Elasticsearch database
	statefulSet, err := c.Client.AppsV1beta1().StatefulSets(elasticsearch.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}

	if statefulSet.Labels[tapi.LabelDatabaseKind] != tapi.ResourceKindElasticsearch ||
		statefulSet.Labels[tapi.LabelDatabaseName] != elasticsearchName {
		return fmt.Errorf(`intended statefulSet "%v" already exists`, name)
	}

	return nil
}

func upsertObjectMeta(statefulSet *apps.StatefulSet, labels, annotations map[string]string) *apps.StatefulSet {
	statefulSet.Labels = kutilcore.UpsertMap(statefulSet.Labels, labels)
	statefulSet.Annotations = kutilcore.UpsertMap(statefulSet.Annotations, annotations)
	return statefulSet
}

func upsertInitContainer(statefulSet *apps.StatefulSet) *apps.StatefulSet {
	container := core.Container{
		Name:            "init-sysctl",
		Image:           "busybox",
		ImagePullPolicy: core.PullIfNotPresent,
		Command:         []string{"sysctl", "-w", "vm.max_map_count=262144"},
		SecurityContext: &core.SecurityContext{
			Privileged: types.BoolP(true),
		},
	}
	initContainers := statefulSet.Spec.Template.Spec.InitContainers
	initContainers = kutilcore.UpsertContainer(initContainers, container)
	statefulSet.Spec.Template.Spec.InitContainers = initContainers
	return statefulSet
}

func upsertContainer(statefulSet *apps.StatefulSet, elasticsearch *tapi.Elasticsearch) *apps.StatefulSet {
	container := core.Container{
		Name:            tapi.ResourceNameElasticsearch,
		Image:           fmt.Sprintf("%v:%v", docker.ImageElasticsearch, elasticsearch.Spec.Version),
		ImagePullPolicy: core.PullIfNotPresent,
		SecurityContext: &core.SecurityContext{
			Privileged: types.BoolP(false),
			Capabilities: &core.Capabilities{
				Add: []core.Capability{"IPC_LOCK", "SYS_RESOURCE"},
			},
		},
	}
	containers := statefulSet.Spec.Template.Spec.Containers
	containers = kutilcore.UpsertContainer(containers, container)
	statefulSet.Spec.Template.Spec.Containers = containers
	return statefulSet
}

func upsertEnv(statefulSet *apps.StatefulSet, elasticsearch *tapi.Elasticsearch, envs []core.EnvVar) *apps.StatefulSet {

	envList := []core.EnvVar{
		{
			Name:  "CLUSTER_NAME",
			Value: elasticsearch.Name,
		},
		{
			Name: "NODE_NAME",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "ES_JAVA_OPTS",
			Value: "-Xms512m -Xmx512m",
		},
		{
			Name:  "DISCOVERY_SERVICE",
			Value: elasticsearch.MasterServiceName(),
		},
		{
			Name:  "SSL_ENABLE",
			Value: fmt.Sprintf("%v", elasticsearch.Spec.EnableSSL),
		},
	}

	envList = append(envList, envs...)

	// To do this, Upsert Container first
	for i, container := range statefulSet.Spec.Template.Spec.Containers {
		if container.Name == tapi.ResourceNameElasticsearch {
			statefulSet.Spec.Template.Spec.Containers[i].Env = kutilcore.UpsertEnvVars(container.Env, envList...)
			return statefulSet
		}
	}

	return statefulSet
}

func upsertPort(statefulSet *apps.StatefulSet, isClient bool) *apps.StatefulSet {

	getPorts := func() []core.ContainerPort {
		portList := []core.ContainerPort{
			{
				Name:          "transport",
				ContainerPort: 9300,
				Protocol:      core.ProtocolTCP,
			},
		}
		if isClient {
			portList = append(portList, core.ContainerPort{
				Name:          "http",
				ContainerPort: 9200,
				Protocol:      core.ProtocolTCP,
			})
		}

		return portList
	}

	for i, container := range statefulSet.Spec.Template.Spec.Containers {
		if container.Name == tapi.ResourceNameElasticsearch {
			statefulSet.Spec.Template.Spec.Containers[i].Ports = getPorts()
			return statefulSet
		}
	}

	return statefulSet
}

func upsertMonitoringContainer(statefulSet *apps.StatefulSet, elasticsearch *tapi.Elasticsearch, tag string) *apps.StatefulSet {
	if elasticsearch.Spec.Monitor != nil &&
		elasticsearch.Spec.Monitor.Agent == tapi.AgentCoreosPrometheus &&
		elasticsearch.Spec.Monitor.Prometheus != nil {
		container := core.Container{
			Name: "exporter",
			Args: []string{
				"export",
				fmt.Sprintf("--address=:%d", tapi.PrometheusExporterPortNumber),
				"--v=3",
			},
			Image:           docker.ImageOperator + ":" + tag,
			ImagePullPolicy: core.PullIfNotPresent,
			Ports: []core.ContainerPort{
				{
					Name:          tapi.PrometheusExporterPortName,
					Protocol:      core.ProtocolTCP,
					ContainerPort: int32(tapi.PrometheusExporterPortNumber),
				},
			},
		}
		containers := statefulSet.Spec.Template.Spec.Containers
		containers = kutilcore.UpsertContainer(containers, container)
		statefulSet.Spec.Template.Spec.Containers = containers
	}
	return statefulSet
}

func upsertCertificate(statefulset *apps.StatefulSet, secretName string, isClientNode bool) *apps.StatefulSet {
	addCertVolume := func() *core.SecretVolumeSource {
		svs := &core.SecretVolumeSource{
			SecretName: secretName,
			Items: []core.KeyToPath{
				{
					Key:  "truststore.jks",
					Path: "truststore.jks",
				},
				{
					Key:  "keystore.jks",
					Path: "keystore.jks",
				},
			},
		}

		if isClientNode {
			svs.Items = append(svs.Items, core.KeyToPath{
				Key:  "sgadmin.jks",
				Path: "sgadmin.jks",
			})
		}
		return svs
	}

	for i, container := range statefulset.Spec.Template.Spec.Containers {
		if container.Name == tapi.ResourceNameElasticsearch {
			volumeMount := core.VolumeMount{
				Name:      "certs",
				MountPath: "/elasticsearch/config/certs",
			}
			volumeMounts := container.VolumeMounts
			volumeMounts = kutilcore.UpsertVolumeMount(volumeMounts, volumeMount)
			statefulset.Spec.Template.Spec.Containers[i].VolumeMounts = volumeMounts

			volume := core.Volume{
				Name: "certs",
				VolumeSource: core.VolumeSource{
					Secret: addCertVolume(),
				},
			}
			volumes := statefulset.Spec.Template.Spec.Volumes
			volumes = kutilcore.UpsertVolume(volumes, volume)
			statefulset.Spec.Template.Spec.Volumes = volumes
			return statefulset
		}
	}
	return statefulset
}

func upsertDatabaseSecret(statefulset *apps.StatefulSet, secretName string) *apps.StatefulSet {
	for i, container := range statefulset.Spec.Template.Spec.Containers {
		if container.Name == tapi.ResourceNameElasticsearch {
			volumeMount := core.VolumeMount{
				Name:      "sgconfig",
				MountPath: "/elasticsearch/plugins/search-guard-5/sgconfig",
			}
			volumeMounts := container.VolumeMounts
			volumeMounts = kutilcore.UpsertVolumeMount(volumeMounts, volumeMount)
			statefulset.Spec.Template.Spec.Containers[i].VolumeMounts = volumeMounts

			volume := core.Volume{
				Name: "sgconfig",
				VolumeSource: core.VolumeSource{
					Secret: &core.SecretVolumeSource{
						SecretName: secretName,
					},
				},
			}
			volumes := statefulset.Spec.Template.Spec.Volumes
			volumes = kutilcore.UpsertVolume(volumes, volume)
			statefulset.Spec.Template.Spec.Volumes = volumes
			return statefulset
		}
	}
	return statefulset
}

func upsertDataVolume(statefulSet *apps.StatefulSet, elasticsearch *tapi.Elasticsearch) *apps.StatefulSet {
	for i, container := range statefulSet.Spec.Template.Spec.Containers {
		if container.Name == tapi.ResourceNameElasticsearch {
			volumeMount := core.VolumeMount{
				Name:      "data",
				MountPath: "/data",
			}
			volumeMounts := container.VolumeMounts
			volumeMounts = kutilcore.UpsertVolumeMount(volumeMounts, volumeMount)
			statefulSet.Spec.Template.Spec.Containers[i].VolumeMounts = volumeMounts

			pvcSpec := elasticsearch.Spec.Storage
			if pvcSpec != nil {
				if len(pvcSpec.AccessModes) == 0 {
					pvcSpec.AccessModes = []core.PersistentVolumeAccessMode{
						core.ReadWriteOnce,
					}
					log.Infof(`Using "%v" as AccessModes in "%v"`, core.ReadWriteOnce, *pvcSpec)
				}

				volumeClaim := core.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
						Annotations: map[string]string{
							"volume.beta.kubernetes.io/storage-class": *pvcSpec.StorageClassName,
						},
					},
					Spec: *pvcSpec,
				}
				volumeClaims := statefulSet.Spec.VolumeClaimTemplates
				volumeClaims = kutilcore.UpsertVolumeClaim(volumeClaims, volumeClaim)
				statefulSet.Spec.VolumeClaimTemplates = volumeClaims
			} else {
				// Attach Empty directory
				statefulSet.Spec.Template.Spec.Volumes = append(
					statefulSet.Spec.Template.Spec.Volumes,
					core.Volume{
						Name: "data",
						VolumeSource: core.VolumeSource{
							EmptyDir: &core.EmptyDirVolumeSource{},
						},
					},
				)
				volume := core.Volume{
					Name: "data",
					VolumeSource: core.VolumeSource{
						EmptyDir: &core.EmptyDirVolumeSource{},
					},
				}
				volumes := statefulSet.Spec.Template.Spec.Volumes
				volumes = kutilcore.UpsertVolume(volumes, volume)
				statefulSet.Spec.Template.Spec.Volumes = volumes
				return statefulSet

			}
		}
	}
	return statefulSet
}

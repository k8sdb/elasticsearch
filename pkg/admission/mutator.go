package admission

import (
	"fmt"
	"sync"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	mon_api "github.com/appscode/kube-mon/api"
	hookapi "github.com/appscode/kubernetes-webhook-util/admission/v1beta1"
	"github.com/appscode/kutil"
	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned"
	"github.com/pkg/errors"
	admission "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ElasticsearchMutator struct {
	client      kubernetes.Interface
	extClient   cs.Interface
	lock        sync.RWMutex
	initialized bool
}

var _ hookapi.AdmissionHook = &ElasticsearchMutator{}

func (a *ElasticsearchMutator) Resource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "mutators.kubedb.com",
			Version:  "v1alpha1",
			Resource: "elasticsearches",
		},
		"elasticsearch"
}

func (a *ElasticsearchMutator) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.initialized = true

	var err error
	if a.client, err = kubernetes.NewForConfig(config); err != nil {
		return err
	}
	if a.extClient, err = cs.NewForConfig(config); err != nil {
		return err
	}
	return err
}

func (a *ElasticsearchMutator) Admit(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}

	// N.B.: No Mutating for delete
	if (req.Operation != admission.Create && req.Operation != admission.Update) ||
		len(req.SubResource) != 0 ||
		req.Kind.Group != api.SchemeGroupVersion.Group ||
		req.Kind.Kind != api.ResourceKindElasticsearch {
		status.Allowed = true
		return status
	}

	a.lock.RLock()
	defer a.lock.RUnlock()
	if !a.initialized {
		return hookapi.StatusUninitialized()
	}
	obj, err := meta_util.UnmarshalFromJSON(req.Object.Raw, api.SchemeGroupVersion)
	if err != nil {
		return hookapi.StatusBadRequest(err)
	}
	mongoMod, err := setDefaultValues(a.client, a.extClient, obj.(*api.Elasticsearch).DeepCopy())
	if err != nil {
		return hookapi.StatusForbidden(err)
	} else if mongoMod != nil {
		patch, err := meta_util.CreateJSONPatch(obj, mongoMod)
		if err != nil {
			return hookapi.StatusInternalServerError(err)
		}
		status.Patch = patch
		patchType := admission.PatchTypeJSONPatch
		status.PatchType = &patchType
	}

	status.Allowed = true
	return status
}

// setDefaultValues provides the defaulting that is performed in mutating stage of creating/updating a Elasticsearch database
func setDefaultValues(client kubernetes.Interface, extClient cs.Interface, elasticsearch *api.Elasticsearch) (runtime.Object, error) {
	if elasticsearch.Spec.Version == "" {
		return nil, fmt.Errorf(`object 'Version' is missing in '%v'`, elasticsearch.Spec)
	}

	topology := elasticsearch.Spec.Topology
	if topology != nil {
		if topology.Client.Replicas == nil {
			topology.Client.Replicas = types.Int32P(1)
		}

		// If resource.request of topology.client is not given, take resource.limit.
		// If, resource.limit is not given either, set default resource.requests to 1Gi.
		// default heapsize is 1G. ref: https://www.elastic.co/guide/en/elasticsearch/reference/current/heap-size.html
		if req, _ := topology.Client.Resources.Requests[core.ResourceMemory]; req.Value() <= 0 {
			if topology.Client.Resources.Requests == nil {
				topology.Client.Resources.Requests = make(core.ResourceList)
			}
			if limit, found := topology.Client.Resources.Limits[core.ResourceMemory]; found && limit.Value() > 0 {
				topology.Client.Resources.Requests[core.ResourceMemory] = limit
			} else {
				topology.Client.Resources.Requests[core.ResourceMemory] = resource.MustParse("1Gi")
			}
		}

		if topology.Master.Replicas == nil {
			topology.Master.Replicas = types.Int32P(1)
		}

		if req, _ := topology.Master.Resources.Requests[core.ResourceMemory]; req.Value() <= 0 {
			if topology.Master.Resources.Requests == nil {
				topology.Master.Resources.Requests = make(core.ResourceList)
			}
			if limit, found := topology.Master.Resources.Limits[core.ResourceMemory]; found && limit.Value() > 0 {
				topology.Master.Resources.Requests[core.ResourceMemory] = limit
			} else {
				topology.Master.Resources.Requests[core.ResourceMemory] = resource.MustParse("1Gi")
			}
		}

		if topology.Data.Replicas == nil {
			topology.Data.Replicas = types.Int32P(1)
		}

		if req, _ := topology.Data.Resources.Requests[core.ResourceMemory]; req.Value() <= 0 {
			if topology.Data.Resources.Requests == nil {
				topology.Data.Resources.Requests = make(core.ResourceList)
			}
			if limit, found := topology.Data.Resources.Limits[core.ResourceMemory]; found && limit.Value() > 0 {
				topology.Data.Resources.Requests[core.ResourceMemory] = limit
			} else {
				topology.Data.Resources.Requests[core.ResourceMemory] = resource.MustParse("1Gi")
			}
		}
	} else {
		if elasticsearch.Spec.Replicas == nil {
			elasticsearch.Spec.Replicas = types.Int32P(1)
		}
		// If resource.request is not given, take resource.limit.
		// If, resource.limit is not given either, set default resource.requests to 1Gi.
		// default heapsize is 1GB. ref: https://www.elastic.co/guide/en/elasticsearch/reference/current/heap-size.html
		if req, _ := elasticsearch.Spec.Resources.Requests[core.ResourceMemory]; req.Value() <= 0 {
			if elasticsearch.Spec.Resources.Requests == nil {
				elasticsearch.Spec.Resources.Requests = make(core.ResourceList)
			}
			if limit, found := elasticsearch.Spec.Resources.Limits[core.ResourceMemory]; found && limit.Value() > 0 {
				elasticsearch.Spec.Resources.Requests[core.ResourceMemory] = limit
			} else {
				elasticsearch.Spec.Resources.Requests[core.ResourceMemory] = resource.MustParse("1Gi")
			}
		}
	}

	if err := setDefaultsFromDormantDB(extClient, elasticsearch); err != nil {
		return nil, err
	}

	// If monitoring spec is given without port,
	// set default Listening port
	setMonitoringPort(elasticsearch)

	return elasticsearch, nil
}

// setDefaultsFromDormantDB takes values from Similar Dormant Database
func setDefaultsFromDormantDB(extClient cs.Interface, elasticsearch *api.Elasticsearch) error {
	// Check if DormantDatabase exists or not
	dormantDb, err := extClient.KubedbV1alpha1().DormantDatabases(elasticsearch.Namespace).Get(elasticsearch.Name, metav1.GetOptions{})
	if err != nil {
		if !kerr.IsNotFound(err) {
			return err
		}
		return nil
	}

	// Check DatabaseKind
	if value, _ := meta_util.GetStringValue(dormantDb.Labels, api.LabelDatabaseKind); value != api.ResourceKindElasticsearch {
		return errors.New(fmt.Sprintf(`invalid Elasticsearch: "%v". Exists DormantDatabase "%v" of different Kind`, elasticsearch.Name, dormantDb.Name))
	}

	// Check Origin Spec
	ddbOriginSpec := dormantDb.Spec.Origin.Spec.Elasticsearch

	// If DatabaseSecret of new object is not given,
	// Take dormantDatabaseSecretName
	if elasticsearch.Spec.DatabaseSecret == nil {
		elasticsearch.Spec.DatabaseSecret = ddbOriginSpec.DatabaseSecret
	}

	// If CertificateSecret of new object is not given,
	// Take dormantDatabase CertificateSecret
	if elasticsearch.Spec.CertificateSecret == nil {
		elasticsearch.Spec.CertificateSecret = ddbOriginSpec.CertificateSecret
	}

	// If Monitoring Spec of new object is not given,
	// Take Monitoring Settings from Dormant
	if elasticsearch.Spec.Monitor == nil {
		elasticsearch.Spec.Monitor = ddbOriginSpec.Monitor
	} else {
		ddbOriginSpec.Monitor = elasticsearch.Spec.Monitor
	}

	// If Backup Scheduler of new object is not given,
	// Take Backup Scheduler Settings from Dormant
	if elasticsearch.Spec.BackupSchedule == nil {
		elasticsearch.Spec.BackupSchedule = ddbOriginSpec.BackupSchedule
	} else {
		ddbOriginSpec.BackupSchedule = elasticsearch.Spec.BackupSchedule
	}

	// Skip checking DoNotPause
	ddbOriginSpec.DoNotPause = elasticsearch.Spec.DoNotPause

	if !meta_util.Equal(ddbOriginSpec, &elasticsearch.Spec) {
		diff := meta_util.Diff(ddbOriginSpec, &elasticsearch.Spec)
		log.Errorf("elasticsearch spec mismatches with OriginSpec in DormantDatabases. Diff: %v", diff)
		return errors.New(fmt.Sprintf("elasticsearch spec mismatches with OriginSpec in DormantDatabases. Diff: %v", diff))
	}

	if _, err := meta_util.GetString(elasticsearch.Annotations, api.AnnotationInitialized); err == kutil.ErrNotFound &&
		elasticsearch.Spec.Init != nil &&
		elasticsearch.Spec.Init.SnapshotSource != nil {
		elasticsearch.Annotations = core_util.UpsertMap(elasticsearch.Annotations, map[string]string{
			api.AnnotationInitialized: "",
		})
	}

	// Delete  Matching dormantDatabase in Controller

	return nil
}

// Assign Default Monitoring Port if MonitoringSpec Exists
// and the AgentVendor is Prometheus.
func setMonitoringPort(elasticsearch *api.Elasticsearch) {
	if elasticsearch.Spec.Monitor != nil &&
		elasticsearch.GetMonitoringVendor() == mon_api.VendorPrometheus {
		if elasticsearch.Spec.Monitor.Prometheus == nil {
			elasticsearch.Spec.Monitor.Prometheus = &mon_api.PrometheusSpec{}
		}
		if elasticsearch.Spec.Monitor.Prometheus.Port == 0 {
			elasticsearch.Spec.Monitor.Prometheus.Port = api.PrometheusExporterPortNumber
		}
	}
}

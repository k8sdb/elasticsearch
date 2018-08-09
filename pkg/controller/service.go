package controller

import (
	"fmt"

	"github.com/appscode/go/log"
	"github.com/appscode/kutil"
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/eventer"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
)

var (
	NodeRoleMaster = "node.role.master"
	NodeRoleClient = "node.role.client"
	NodeRoleData   = "node.role.data"
)

const (
	ElasticsearchRestPort     = 9200
	ElasticsearchRestPortName = "http"
	ElasticsearchNodePort     = 9300
	ElasticsearchNodePortName = "transport"
)

func (c *Controller) ensureService(elasticsearch *api.Elasticsearch) (kutil.VerbType, error) {
	// Check if service name exists
	err := c.checkService(elasticsearch, elasticsearch.OffshootName())
	if err != nil {
		return kutil.VerbUnchanged, err
	}
	// create database Service
	vt1, err := c.createService(elasticsearch)
	if err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				"Failed to createOrPatch Service. Reason: %v",
				err,
			)
		}
		return kutil.VerbUnchanged, err
	} else if vt1 != kutil.VerbUnchanged {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeNormal,
				eventer.EventReasonSuccessful,
				"Successfully %s Service",
				vt1,
			)
		}
	}

	// Check if service name exists
	err = c.checkService(elasticsearch, elasticsearch.MasterServiceName())
	if err != nil {
		return kutil.VerbUnchanged, err
	}
	// create database Service
	vt2, err := c.createMasterService(elasticsearch)
	if err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				"Failed to createOrPatch Service. Reason: %v",
				err,
			)
		}
		return kutil.VerbUnchanged, err
	} else if vt2 != kutil.VerbUnchanged {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeNormal,
				eventer.EventReasonSuccessful,
				"Successfully %s Service",
				vt2,
			)
		}
	}

	if vt1 == kutil.VerbCreated && vt2 == kutil.VerbCreated {
		return kutil.VerbCreated, nil
	} else if vt1 == kutil.VerbPatched || vt2 == kutil.VerbPatched {
		return kutil.VerbPatched, nil
	}

	return kutil.VerbUnchanged, nil
}

func (c *Controller) checkService(elasticsearch *api.Elasticsearch, name string) error {
	service, err := c.Client.CoreV1().Services(elasticsearch.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	if service.Labels[api.LabelDatabaseKind] != api.ResourceKindElasticsearch ||
		service.Labels[api.LabelDatabaseName] != elasticsearch.Name {
		return fmt.Errorf(`intended service "%v" already exists`, name)
	}

	return nil
}

func (c *Controller) createService(elasticsearch *api.Elasticsearch) (kutil.VerbType, error) {
	meta := metav1.ObjectMeta{
		Name:      elasticsearch.OffshootName(),
		Namespace: elasticsearch.Namespace,
	}

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch)
	if rerr != nil {
		return kutil.VerbUnchanged, rerr
	}

	_, ok, err := core_util.CreateOrPatchService(c.Client, meta, func(in *core.Service) *core.Service {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = elasticsearch.OffshootLabels()
		in.Annotations = elasticsearch.Spec.ServiceTemplate.Annotations

		in.Spec.Selector = elasticsearch.OffshootSelectors()
		in.Spec.Selector[NodeRoleClient] = "set"
		in.Spec.Ports = core_util.MergeServicePorts(in.Spec.Ports, []core.ServicePort{
			{
				Name:       ElasticsearchRestPortName,
				Port:       ElasticsearchRestPort,
				TargetPort: intstr.FromString(ElasticsearchRestPortName),
			},
		})

		if elasticsearch.Spec.ServiceTemplate.Spec.ClusterIP != "" {
			in.Spec.ClusterIP = elasticsearch.Spec.ServiceTemplate.Spec.ClusterIP
		}
		if elasticsearch.Spec.ServiceTemplate.Spec.Type != "" {
			in.Spec.Type = elasticsearch.Spec.ServiceTemplate.Spec.Type
		}
		in.Spec.ExternalIPs = elasticsearch.Spec.ServiceTemplate.Spec.ExternalIPs
		in.Spec.LoadBalancerIP = elasticsearch.Spec.ServiceTemplate.Spec.LoadBalancerIP
		in.Spec.LoadBalancerSourceRanges = elasticsearch.Spec.ServiceTemplate.Spec.LoadBalancerSourceRanges
		in.Spec.ExternalTrafficPolicy = elasticsearch.Spec.ServiceTemplate.Spec.ExternalTrafficPolicy
		if elasticsearch.Spec.ServiceTemplate.Spec.HealthCheckNodePort > 0 {
			in.Spec.HealthCheckNodePort = elasticsearch.Spec.ServiceTemplate.Spec.HealthCheckNodePort
		}
		return in
	})
	return ok, err
}

func (c *Controller) createMasterService(elasticsearch *api.Elasticsearch) (kutil.VerbType, error) {
	meta := metav1.ObjectMeta{
		Name:      elasticsearch.MasterServiceName(),
		Namespace: elasticsearch.Namespace,
	}

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch)
	if rerr != nil {
		return kutil.VerbUnchanged, rerr
	}

	_, ok, err := core_util.CreateOrPatchService(c.Client, meta, func(in *core.Service) *core.Service {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = elasticsearch.OffshootLabels()
		in.Annotations = elasticsearch.Spec.ServiceTemplate.Annotations

		in.Spec.Selector = elasticsearch.OffshootSelectors()
		in.Spec.Selector[NodeRoleMaster] = "set"
		in.Spec.Ports = core_util.MergeServicePorts(in.Spec.Ports, []core.ServicePort{
			{
				Name:       ElasticsearchNodePortName,
				Port:       ElasticsearchNodePort,
				TargetPort: intstr.FromString(ElasticsearchNodePortName),
			},
		})
		return in
	})
	return ok, err
}

func (c *Controller) ensureStatsService(elasticsearch *api.Elasticsearch) (kutil.VerbType, error) {
	// return if monitoring is not prometheus
	if elasticsearch.GetMonitoringVendor() != mona.VendorPrometheus {
		log.Warningln("elasticsearch.spec.monitor.agent is not coreos-operator or builtin.")
		return kutil.VerbUnchanged, nil
	}

	// Check if statsService name exists
	if err := c.checkService(elasticsearch, elasticsearch.StatsService().ServiceName()); err != nil {
		return kutil.VerbUnchanged, err
	}

	// create statsService
	vt, err := c.createStatsService(elasticsearch)
	if err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				"Failed to create StatsService. Reason: %v",
				err,
			)
		}
		return kutil.VerbUnchanged, err
	} else if vt != kutil.VerbUnchanged {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeNormal,
				eventer.EventReasonSuccessful,
				"Successfully %s StatsService",
				vt,
			)
		}
	}
	return vt, nil
}

func (c *Controller) createStatsService(elasticsearch *api.Elasticsearch) (kutil.VerbType, error) {
	meta := metav1.ObjectMeta{
		Name:      elasticsearch.StatsService().ServiceName(),
		Namespace: elasticsearch.Namespace,
	}
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, elasticsearch)
	if rerr != nil {
		return kutil.VerbUnchanged, rerr
	}
	_, ok, err := core_util.CreateOrPatchService(c.Client, meta, func(in *core.Service) *core.Service {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = elasticsearch.OffshootLabels()
		in.Spec.Selector = elasticsearch.OffshootSelectors()
		in.Spec.Ports = core_util.MergeServicePorts(in.Spec.Ports, []core.ServicePort{
			{
				Name:       api.PrometheusExporterPortName,
				Protocol:   core.ProtocolTCP,
				Port:       elasticsearch.Spec.Monitor.Prometheus.Port,
				TargetPort: intstr.FromString(api.PrometheusExporterPortName),
			},
		})
		return in
	})
	return ok, err
}

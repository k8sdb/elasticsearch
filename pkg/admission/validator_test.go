/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admission

import (
	"context"
	"net/http"
	"testing"

	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	extFake "kubedb.dev/apimachinery/client/clientset/versioned/fake"
	"kubedb.dev/apimachinery/client/clientset/versioned/scheme"

	"gomodules.xyz/pointer"
	admission "k8s.io/api/admission/v1beta1"
	authenticationV1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	storageV1beta1 "k8s.io/api/storage/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientSetScheme "k8s.io/client-go/kubernetes/scheme"
	core_util "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
	ofst "kmodules.xyz/offshoot-api/api/v1"
)

func init() {
	utilRuntime.Must(scheme.AddToScheme(clientSetScheme.Scheme))
}

var requestKind = metaV1.GroupVersionKind{
	Group:   api.SchemeGroupVersion.Group,
	Version: api.SchemeGroupVersion.Version,
	Kind:    api.ResourceKindElasticsearch,
}

func TestElasticsearchValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			validator := ElasticsearchValidator{
				ClusterTopology: &core_util.Topology{},
			}

			validator.initialized = true
			validator.client = fake.NewSimpleClientset()
			validator.extClient = extFake.NewSimpleClientset(
				&catalog.ElasticsearchVersion{
					ObjectMeta: metaV1.ObjectMeta{
						Name: "5.6",
					},
				},
			)
			validator.client = fake.NewSimpleClientset(
				&core.Secret{
					ObjectMeta: metaV1.ObjectMeta{
						Name:      "foo-auth",
						Namespace: "default",
					},
				},
				&storageV1beta1.StorageClass{
					ObjectMeta: metaV1.ObjectMeta{
						Name: "standard",
					},
				},
			)

			objJS, err := meta_util.MarshalToJson(&c.object, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}
			oldObjJS, err := meta_util.MarshalToJson(&c.oldObject, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}

			req := new(admission.AdmissionRequest)

			req.Kind = c.kind
			req.Name = c.objectName
			req.Namespace = c.namespace
			req.Operation = c.operation
			req.UserInfo = authenticationV1.UserInfo{}
			req.Object.Raw = objJS
			req.OldObject.Raw = oldObjJS

			if c.heatUp {
				if _, err := validator.extClient.KubedbV1alpha2().Elasticsearches(c.namespace).Create(context.TODO(), &c.object, metaV1.CreateOptions{}); err != nil && !kerr.IsAlreadyExists(err) {
					t.Errorf(err.Error())
				}
			}
			if c.operation == admission.Delete {
				req.Object = runtime.RawExtension{}
			}
			if c.operation != admission.Update {
				req.OldObject = runtime.RawExtension{}
			}

			response := validator.Admit(req)
			if c.result == true {
				if response.Allowed != true {
					t.Errorf("expected: 'Allowed=true'. but got response: %v", response)
				}
			} else if c.result == false {
				if response.Allowed == true || response.Result.Code == http.StatusInternalServerError {
					t.Errorf("expected: 'Allowed=false', but got response: %v", response)
				}
			}
		})
	}

}

var cases = []struct {
	testName   string
	kind       metaV1.GroupVersionKind
	objectName string
	namespace  string
	operation  admission.Operation
	object     api.Elasticsearch
	oldObject  api.Elasticsearch
	heatUp     bool
	result     bool
}{
	{"Create Valid Elasticsearch",
		requestKind,
		"foo",
		"default",
		admission.Create,
		sampleElasticsearch(),
		api.Elasticsearch{},
		false,
		true,
	},
	{"Create Invalid Elasticsearch",
		requestKind,
		"foo",
		"default",
		admission.Create,
		getAwkwardElasticsearch(),
		api.Elasticsearch{},
		false,
		false,
	},
	{"Edit Elasticsearch Spec.AuthSecret with Existing Secret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editExistingSecret(sampleElasticsearch()),
		sampleElasticsearch(),
		false,
		true,
	},
	{"Edit Elasticsearch Spec.AuthSecret with non Existing Secret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editNonExistingSecret(sampleElasticsearch()),
		sampleElasticsearch(),
		false,
		true,
	},
	{"Edit Status",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editStatus(sampleElasticsearch()),
		sampleElasticsearch(),
		false,
		true,
	},
	{"Edit Spec.Monitor",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editSpecMonitor(sampleElasticsearch()),
		sampleElasticsearch(),
		false,
		true,
	},
	{"Edit Invalid Spec.Monitor",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editSpecInvalidMonitor(sampleElasticsearch()),
		sampleElasticsearch(),
		false,
		false,
	},
	{"Edit Spec.TerminationPolicy",
		requestKind,
		"foo",
		"default",
		admission.Update,
		haltDatabase(sampleElasticsearch()),
		sampleElasticsearch(),
		false,
		true,
	},
	{"Delete Elasticsearch when Spec.TerminationPolicy=DoNotTerminate",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		sampleElasticsearch(),
		api.Elasticsearch{},
		true,
		false,
	},
	{"Delete Elasticsearch when Spec.TerminationPolicy=Halt",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		haltDatabase(sampleElasticsearch()),
		api.Elasticsearch{},
		true,
		true,
	},
	{"Delete Non Existing Elasticsearch",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		api.Elasticsearch{},
		api.Elasticsearch{},
		false,
		true,
	},
	{"Edit spec.Init before provisioning complete",
		requestKind,
		"foo",
		"default",
		admission.Update,
		updateInit(sampleElasticsearch()),
		sampleElasticsearch(),
		true,
		true,
	},
	{"Edit spec.Init after provisioning complete",
		requestKind,
		"foo",
		"default",
		admission.Update,
		updateInit(completeInitialization(sampleElasticsearch())),
		completeInitialization(sampleElasticsearch()),
		true,
		false,
	},
	{
		testName:   "Create elasticsearch with low resource request",
		kind:       requestKind,
		objectName: "foo",
		namespace:  "default",
		operation:  admission.Create,
		object: transformElasticsearch(sampleElasticsearch(), func(in api.Elasticsearch) api.Elasticsearch {
			in.Spec.PodTemplate.Spec.Resources.Requests[core.ResourceMemory] = resource.MustParse("250Mi")
			return in
		}),
		heatUp: true,
		result: false,
	},
}

func sampleElasticsearch() api.Elasticsearch {
	return api.Elasticsearch{
		TypeMeta: metaV1.TypeMeta{
			Kind:       api.ResourceKindElasticsearch,
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				meta_util.NameLabelKey: api.Elasticsearch{}.ResourceFQN(),
			},
		},
		Spec: api.ElasticsearchSpec{
			Version:     "5.6",
			Replicas:    pointer.Int32P(1),
			StorageType: api.StorageTypeDurable,
			Storage: &core.PersistentVolumeClaimSpec{
				StorageClassName: pointer.StringP("standard"),
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse("100Mi"),
					},
				},
			},
			PodTemplate: ofst.PodTemplateSpec{
				Spec: ofst.PodSpec{
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
			Init: &api.InitSpec{
				WaitForInitialRestore: true,
			},
			TerminationPolicy: api.TerminationPolicyDoNotTerminate,
		},
	}
}

func getAwkwardElasticsearch() api.Elasticsearch {
	elasticsearch := sampleElasticsearch()
	elasticsearch.Spec.Version = "3.0"
	return elasticsearch
}

func editExistingSecret(old api.Elasticsearch) api.Elasticsearch {
	old.Spec.AuthSecret = &core.LocalObjectReference{
		Name: "foo-auth",
	}
	return old
}

func transformElasticsearch(es api.Elasticsearch, transform func(in api.Elasticsearch) api.Elasticsearch) api.Elasticsearch {
	return transform(es)
}

func editNonExistingSecret(old api.Elasticsearch) api.Elasticsearch {
	old.Spec.AuthSecret = &core.LocalObjectReference{
		Name: "foo-auth-fused",
	}
	return old
}

func editStatus(old api.Elasticsearch) api.Elasticsearch {
	old.Status = api.ElasticsearchStatus{
		Phase: api.DatabasePhaseReady,
	}
	return old
}

func editSpecMonitor(old api.Elasticsearch) api.Elasticsearch {
	old.Spec.Monitor = &mona.AgentSpec{
		Agent: mona.AgentPrometheusBuiltin,
		Prometheus: &mona.PrometheusSpec{
			Exporter: mona.PrometheusExporterSpec{
				Port: 1289,
			},
		},
	}
	return old
}

// should be failed because more fields required for COreOS Monitoring
func editSpecInvalidMonitor(old api.Elasticsearch) api.Elasticsearch {
	old.Spec.Monitor = &mona.AgentSpec{
		Agent: mona.AgentPrometheusOperator,
	}
	return old
}

func haltDatabase(old api.Elasticsearch) api.Elasticsearch {
	old.Spec.TerminationPolicy = api.TerminationPolicyHalt
	return old
}

func completeInitialization(old api.Elasticsearch) api.Elasticsearch {
	old.Spec.Init.Initialized = true
	return old
}

func updateInit(old api.Elasticsearch) api.Elasticsearch {
	old.Spec.Init.WaitForInitialRestore = false
	return old
}

/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"kubedb.dev/apimachinery/apis/catalog/v1alpha1"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kmapi "kmodules.xyz/client-go/api/v1"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
	ofst "kmodules.xyz/offshoot-api/api/v1"
)

const (
	ResourceCodeElasticsearch     = "es"
	ResourceKindElasticsearch     = "Elasticsearch"
	ResourceSingularElasticsearch = "elasticsearch"
	ResourcePluralElasticsearch   = "elasticsearches"
)

// Elasticsearch defines a Elasticsearch database.

// +genclient
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=elasticsearches,singular=elasticsearch,shortName=es,categories={datastore,kubedb,appscode,all}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Elasticsearch struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ElasticsearchSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            ElasticsearchStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

type ElasticsearchSpec struct {
	// Version of Elasticsearch to be deployed.
	Version string `json:"version" protobuf:"bytes,1,opt,name=version"`

	// Number of instances to deploy for a Elasticsearch database.
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,2,opt,name=replicas"`

	// Elasticsearch topology for node specification
	Topology *ElasticsearchClusterTopology `json:"topology,omitempty" protobuf:"bytes,3,opt,name=topology"`

	// To enable ssl in transport & http layer
	EnableSSL bool `json:"enableSSL,omitempty" protobuf:"varint,4,opt,name=enableSSL"`

	// Secret with SSL certificates
	// Deprecated: Use spec.tls instead
	CertificateSecret *core.SecretVolumeSource `json:"certificateSecret,omitempty" protobuf:"bytes,5,opt,name=certificateSecret"`

	// disable security of authPlugin (ie, xpack or searchguard). It disables authentication security of user.
	// If unset, default is false
	DisableSecurity bool `json:"disableSecurity,omitempty" protobuf:"varint,6,opt,name=disableSecurity"`

	// Authentication plugin used by Elasticsearch cluster. If unset, defaults to SearchGuard.
	// Deprecated: Use elasticsearchVersion.Spec.AuthPlugin instead
	AuthPlugin v1alpha1.ElasticsearchAuthPlugin `json:"authPlugin,omitempty" protobuf:"bytes,7,opt,name=authPlugin,casttype=kubedb.dev/apimachinery/apis/catalog/v1alpha1.ElasticsearchAuthPlugin"`

	// Database authentication secret
	DatabaseSecret *core.SecretVolumeSource `json:"databaseSecret,omitempty" protobuf:"bytes,8,opt,name=databaseSecret"`

	// StorageType can be durable (default) or ephemeral
	StorageType StorageType `json:"storageType,omitempty" protobuf:"bytes,9,opt,name=storageType,casttype=StorageType"`

	// Storage to specify how storage shall be used.
	Storage *core.PersistentVolumeClaimSpec `json:"storage,omitempty" protobuf:"bytes,10,opt,name=storage"`

	// Init is used to initialize database
	// +optional
	Init *InitSpec `json:"init,omitempty" protobuf:"bytes,11,opt,name=init"`

	// Monitor is used monitor database instance
	// +optional
	Monitor *mona.AgentSpec `json:"monitor,omitempty" protobuf:"bytes,12,opt,name=monitor"`

	// ConfigSource is an optional field to provide custom configuration file for database.
	// If specified, this file will be used as configuration file otherwise default configuration file will be used.
	ConfigSource *core.VolumeSource `json:"configSource,omitempty" protobuf:"bytes,13,opt,name=configSource"`

	// PodTemplate is an optional configuration for pods used to expose database
	// +optional
	PodTemplate ofst.PodTemplateSpec `json:"podTemplate,omitempty" protobuf:"bytes,14,opt,name=podTemplate"`

	// ServiceTemplate is an optional configuration for service used to expose database
	// +optional
	ServiceTemplate ofst.ServiceTemplateSpec `json:"serviceTemplate,omitempty" protobuf:"bytes,15,opt,name=serviceTemplate"`

	// An eviction is allowed if at most "maxUnavailable" pods selected by
	// "selector" are unavailable after the eviction, i.e. even in absence of
	// the evicted pod. For example, one can prevent all voluntary evictions
	// by specifying 0. This is a mutually exclusive setting with "minAvailable".
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty" protobuf:"bytes,16,opt,name=maxUnavailable"`

	// updateStrategy indicates the StatefulSetUpdateStrategy that will be
	// employed to update Pods in the StatefulSet when a revision is made to
	// Template.
	// Deprecated: UpdateStrategy is default to "OnDelete"
	UpdateStrategy apps.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty" protobuf:"bytes,17,opt,name=updateStrategy"`

	// TLS contains tls configurations
	// +optional
	TLS *kmapi.TLSConfig `json:"tls,omitempty" protobuf:"bytes,18,opt,name=tls"`

	// InternalUsers contains internal user configurations
	// Expected Input format:
	// internalUsers:
	//   <username1>:
	//		...
	//   <username2>:
	//		...
	// +optional
	InternalUsers map[string]ElasticsearchUserSpec `json:"internalUsers,omitempty" protobuf:"bytes,19,rep,name=internalUsers"`

	// Indicates that the database is paused and controller will not sync any changes made to this spec.
	// +optional
	Paused bool `json:"paused,omitempty" protobuf:"varint,20,opt,name=paused"`

	// Indicates that the database is halted and all offshoot Kubernetes resources except PVCs are deleted.
	// +optional
	Halted bool `json:"halted,omitempty" protobuf:"varint,21,opt,name=halted"`

	// TerminationPolicy controls the delete operation for database
	// +optional
	TerminationPolicy TerminationPolicy `json:"terminationPolicy,omitempty" protobuf:"bytes,22,opt,name=terminationPolicy,casttype=TerminationPolicy"`
}

type ElasticsearchClusterTopology struct {
	Master ElasticsearchNode `json:"master" protobuf:"bytes,1,opt,name=master"`
	Data   ElasticsearchNode `json:"data" protobuf:"bytes,2,opt,name=data"`
	Client ElasticsearchNode `json:"client" protobuf:"bytes,3,opt,name=client"`
}

type ElasticsearchNode struct {
	// Replicas represents number of replica for this specific type of node
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
	Prefix   string `json:"prefix,omitempty" protobuf:"bytes,2,opt,name=prefix"`
	// Storage to specify how storage shall be used.
	Storage *core.PersistentVolumeClaimSpec `json:"storage,omitempty" protobuf:"bytes,3,opt,name=storage"`
	// Compute Resources required by the sidecar container.
	Resources core.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,4,opt,name=resources"`
	// An eviction is allowed if at most "maxUnavailable" pods selected by
	// "selector" are unavailable after the eviction, i.e. even in absence of
	// the evicted pod. For example, one can prevent all voluntary evictions
	// by specifying 0. This is a mutually exclusive setting with "minAvailable".
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty" protobuf:"bytes,5,opt,name=maxUnavailable"`
}

// +kubebuilder:validation:Enum=root;transport;http;admin;archiver;metrics-exporter
type ElasticsearchCertificateAlias string

const (
	ElasticsearchRootCert            ElasticsearchCertificateAlias = "root"
	ElasticsearchTransportCert       ElasticsearchCertificateAlias = "transport"
	ElasticsearchHTTPCert            ElasticsearchCertificateAlias = "http"
	ElasticsearchAdminCert           ElasticsearchCertificateAlias = "admin"
	ElasticsearchArchiverCert        ElasticsearchCertificateAlias = "archiver"
	ElasticsearchMetricsExporterCert ElasticsearchCertificateAlias = "metrics-exporter"
)

type ElasticsearchInternalUser string

const (
	ElasticsearchInternalUserElastic         ElasticsearchInternalUser = "elastic"
	ElasticsearchInternalUserAdmin           ElasticsearchInternalUser = "admin"
	ElasticsearchInternalUserKibanaserver    ElasticsearchInternalUser = "kibanaserver"
	ElasticsearchInternalUserKibanaro        ElasticsearchInternalUser = "kibanaro"
	ElasticsearchInternalUserLogstash        ElasticsearchInternalUser = "logstash"
	ElasticsearchInternalUserReadall         ElasticsearchInternalUser = "readall"
	ElasticsearchInternalUserSnapshotrestore ElasticsearchInternalUser = "snapshotrestore"
)

// Specifies the security plugin internal user structure.
// Both 'json' and 'yaml' tags are used in structure metadata.
// The `json` tags (camel case) are used while taking input from users.
// The `yaml` tags (snake case) are used by the operator to generate internal_users.yml file.
type ElasticsearchUserSpec struct {
	// Specifies the hash of the password.
	// +optional
	Hash string `json:"-" yaml:"hash,omitempty" protobuf:"bytes,1,opt,name=hash"`

	// Specifies the reserved status.
	// Resources that have this set to true can’t be changed using the REST API or Kibana.
	// Default to "false".
	// +optional
	Reserved bool `json:"reserved,omitempty" yaml:"reserved,omitempty" protobuf:"bytes,2,opt,name=reserved"`

	// Specifies the hidden status.
	// Resources that have this set to true are not returned by the REST API
	// and not visible in Kibana.
	// Default to "false".
	// +optional
	Hidden bool `json:"hidden,omitempty" yaml:"hidden,omitempty" protobuf:"bytes,3,opt,name=hidden"`

	// Specifies a list of backend roles assigned to this user.
	// Backend roles can come from the internal user database,
	// LDAP groups, JSON web token claims or SAML assertions.
	// +optional
	BackendRoles []string `json:"backendRoles,omitempty" yaml:"backend_roles,omitempty" protobuf:"bytes,4,opt,name=backendRoles"`

	// Specifies a list of searchguard security plugin roles assigned to this user.
	// +optional
	SearchGuardRoles []string `json:"searchGuardRoles,omitempty" yaml:"search_guard_roles,omitempty" protobuf:"bytes,5,opt,name=searchGuardRoles"`

	// Specifies a list of opendistro security plugin roles assigned to this user.
	// +optional
	OpendistroSecurityRoles []string `json:"opendistroSecurityRoles,omitempty" yaml:"opendistro_security_roles,omitempty" protobuf:"bytes,6,opt,name=opendistroSecurityRoles"`

	// Specifies one or more custom attributes,
	// which can be used in index names and DLS queries.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty" yaml:"attributes,omitempty" protobuf:"bytes,7,opt,name=attributes"`

	// Specifies the description of the user
	// +optional
	Description string `json:"description,omitempty" yaml:"description,omitempty" protobuf:"bytes,8,opt,name=description"`
}

type ElasticsearchStatus struct {
	Phase  DatabasePhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase,casttype=DatabasePhase"`
	Reason string        `json:"reason,omitempty" protobuf:"bytes,2,opt,name=reason"`
	// observedGeneration is the most recent generation observed for this resource. It corresponds to the
	// resource's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ElasticsearchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is a list of Elasticsearch CRD objects
	Items []Elasticsearch `json:"items,omitempty" protobuf:"bytes,2,rep,name=items"`
}

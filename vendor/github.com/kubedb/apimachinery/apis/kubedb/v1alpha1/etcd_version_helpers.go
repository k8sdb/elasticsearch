package v1alpha1

import (
	crdutils "github.com/appscode/kutil/apiextensions/v1beta1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

var _ ResourceInfo = &EtcdVersion{}

func (p EtcdVersion) ResourceShortCode() string {
	return ResourceCodeEtcdVersion
}

func (p EtcdVersion) ResourceKind() string {
	return ResourceKindEtcdVersion
}

func (p EtcdVersion) ResourceSingular() string {
	return ResourceSingularEtcdVersion
}

func (p EtcdVersion) ResourcePlural() string {
	return ResourcePluralEtcdVersion
}

func (p EtcdVersion) CustomResourceDefinition() *apiextensions.CustomResourceDefinition {
	return crdutils.NewCustomResourceDefinition(crdutils.Config{
		Group:         SchemeGroupVersion.Group,
		Plural:        ResourcePluralEtcdVersion,
		Singular:      ResourceSingularEtcdVersion,
		Kind:          ResourceKindEtcdVersion,
		ShortNames:    []string{ResourceCodeEtcdVersion},
		Categories:    []string{"datastore", "kubedb", "appscode", "all"},
		ResourceScope: string(apiextensions.ClusterScoped),
		Versions: []apiextensions.CustomResourceDefinitionVersion{
			{
				Name:    SchemeGroupVersion.Version,
				Served:  true,
				Storage: true,
			},
		},
		Labels: crdutils.Labels{
			LabelsMap: map[string]string{"app": "kubedb"},
		},
		SpecDefinitionName:      "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1.EtcdVersion",
		EnableValidation:        true,
		GetOpenAPIDefinitions:   GetOpenAPIDefinitions,
		EnableStatusSubresource: false,
		AdditionalPrinterColumns: []apiextensions.CustomResourceColumnDefinition{
			{
				Name:     "Version",
				Type:     "string",
				JSONPath: ".spec.version",
			},
			{
				Name:     "DbImage",
				Type:     "string",
				JSONPath: ".spec.db.image",
			},
			{
				Name:     "ExporterImage",
				Type:     "string",
				JSONPath: ".spec.exporter.image",
			},
			{
				Name:     "Age",
				Type:     "date",
				JSONPath: ".metadata.creationTimestamp",
			},
		},
	})
}

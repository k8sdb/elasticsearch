package es_util

import (
	"crypto/tls"
	"fmt"
	"net/http"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	esv5 "gopkg.in/olivere/elastic.v5"
	esv6 "gopkg.in/olivere/elastic.v6"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	KeyAdminPassword = "ADMIN_PASSWORD"
	AdminUser        = "admin"
)

type ESClient interface {
	CreateIndex(count int) error
	CountIndex() (int, error)
	GetIndexNames() ([]string, error)
	GetAllNodesInfo() ([]NodeInfo, error)
	GetElasticsearchSummary(indexName string) (*api.ElasticsearchSummary, error)
	Stop()
}

type NodeSetting struct {
	Name   string `json:"name,omitempty"`
	Data   string `json:"data,omitempty"`
	Ingest string `json:"ingest,omitempty"`
	Master string `json:"master,omitempty"`
}
type PathSetting struct {
	Data []string `json:"data,omitempty"`
	Logs string   `json:"logs,omitempty"`
	Home string   `json:"home,omitempty"`
}
type Setting struct {
	Node *NodeSetting `json:"node,omitempty"`
	Path *PathSetting `json:"path,omitempty"`
}

type NodeInfo struct {
	Name     string   `json:"name,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	Settings *Setting `json:"settings,omitempty"`
}

func GetElasticClient(kubeClient kubernetes.Interface, elasticsearch *api.Elasticsearch, url string) (ESClient, error) {
	secret, err := kubeClient.CoreV1().Secrets(elasticsearch.Namespace).Get(elasticsearch.Spec.DatabaseSecret.SecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	switch elasticsearch.Spec.Version {
	case "5.6", "5.6.4":
		client, err := esv5.NewClient(
			esv5.SetHttpClient(&http.Client{
				Timeout: 0,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}),
			esv5.SetBasicAuth(AdminUser, string(secret.Data[KeyAdminPassword])),
			esv5.SetURL(url),
			esv5.SetHealthcheck(true),
			esv5.SetSniff(false),
		)
		if err != nil {
			return nil, err
		}

		return &ESClientV5{client: client}, nil
	case "6.2", "6.2.4", "6.3", "6.3.0":
		client, err := esv6.NewClient(
			esv6.SetHttpClient(&http.Client{
				Timeout: 0,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}),
			esv6.SetBasicAuth(AdminUser, string(secret.Data[KeyAdminPassword])),
			esv6.SetURL(url),
			esv6.SetHealthcheck(true),
			esv6.SetSniff(false),
		)
		if err != nil {
			return nil, err
		}
		return &ESClientV6{client: client}, nil
	}
	return nil, fmt.Errorf("unknown database verserion: %s\n", elasticsearch.Spec.Version)
}
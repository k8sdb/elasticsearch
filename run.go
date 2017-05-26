package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	acstr "github.com/appscode/go/strings"
	"github.com/appscode/go/version"
	"github.com/appscode/log"
	pcm "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1alpha1"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	"github.com/k8sdb/apimachinery/pkg/docker"
	"github.com/k8sdb/elasticsearch/pkg/controller"
	"github.com/spf13/cobra"
	cgcmd "k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/util/runtime"
)

func NewCmdRun() *cobra.Command {
	var (
		masterURL      string
		kubeconfigPath string
	)

	opt := &controller.Option{
		ElasticDumpTag:    "canary",
		OperatorTag:       acstr.Val(version.Version.Version, "canary"),
		ExporterNamespace: namespace(),
		ExporterTag:       "canary",
		GoverningService:  "kubedb",
		Address:           ":8080",
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run Elasticsearch in Kubernetes",
		Run: func(cmd *cobra.Command, args []string) {
			config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				log.Fatalf("Could not get kubernetes config: %s", err)
			}

			// Check elasticdump docker image tag

			if err := docker.CheckDockerImageVersion(docker.ImageElasticdump, opt.ElasticDumpTag); err != nil {
				log.Fatalf(`Image %v:%v not found.`, docker.ImageElasticdump, opt.ElasticDumpTag)
			}

			// Check exporter docker image tag
			if err := docker.CheckDockerImageVersion(docker.ImageExporter, opt.ExporterTag); err != nil {
				log.Fatalf(`Image %v:%v not found.`, docker.ImageExporter, opt.ExporterTag)
			}

			client := clientset.NewForConfigOrDie(config)
			extClient := tcs.NewExtensionsForConfigOrDie(config)

			cgConfig, err := cgcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				log.Fatalf("Could not get kubernetes config: %s", err)
			}

			promClient, err := pcm.NewForConfig(cgConfig)
			if err != nil {
				log.Fatalln(err)
			}

			w := controller.New(client, extClient, promClient, opt)
			defer runtime.HandleCrash()
			fmt.Println("Starting operator...")
			w.RunAndHold()
		},
	}

	cmd.Flags().StringVar(&masterURL, "master", "", "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	cmd.Flags().StringVar(&opt.ElasticDumpTag, "elasticdump", opt.ElasticDumpTag, "Tag of elasticdump")
	cmd.Flags().StringVar(&opt.OperatorTag, "operator", opt.OperatorTag, "Tag of elasticsearch opearator")
	cmd.Flags().StringVar(&opt.ExporterNamespace, "exporter-ns", opt.ExporterNamespace, "Namespace for monitoring exporter")
	cmd.Flags().StringVar(&opt.ExporterTag, "exporter", opt.ExporterTag, "Tag of monitoring expoter")
	cmd.Flags().StringVar(&opt.GoverningService, "governing-service", opt.GoverningService, "Governing service for database statefulset")
	cmd.Flags().StringVar(&opt.Address, "address", opt.Address, "Address to listen on for web interface and telemetry.")

	return cmd
}

func namespace() string {
	if ns := os.Getenv("OPERATOR_NAMESPACE"); ns != "" {
		return ns
	}
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}
	return kapi.NamespaceDefault
}

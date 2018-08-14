package e2e_test

import (
	"os"

	exec_util "github.com/appscode/kutil/tools/exec"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/elasticsearch/test/e2e/framework"
	"github.com/kubedb/elasticsearch/test/e2e/matcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	store "kmodules.xyz/objectstore-api/api/v1"
)

const (
	S3_BUCKET_NAME       = "S3_BUCKET_NAME"
	GCS_BUCKET_NAME      = "GCS_BUCKET_NAME"
	AZURE_CONTAINER_NAME = "AZURE_CONTAINER_NAME"
	SWIFT_CONTAINER_NAME = "SWIFT_CONTAINER_NAME"
)

var _ = Describe("Elasticsearch", func() {
	var (
		err                      error
		f                        *framework.Invocation
		elasticsearch            *api.Elasticsearch
		garbageElasticsearch     *api.ElasticsearchList
		elasticsearchVersion     *api.ElasticsearchVersion
		snapshot                 *api.Snapshot
		secret                   *core.Secret
		skipMessage              string
		skipSnapshotDataChecking bool
	)

	BeforeEach(func() {
		f = root.Invoke()
		elasticsearch = f.CombinedElasticsearch()
		elasticsearchVersion = f.ElasticsearchVersion()
		garbageElasticsearch = new(api.ElasticsearchList)
		snapshot = f.Snapshot()
		secret = new(core.Secret)
		skipMessage = ""
		skipSnapshotDataChecking = true
	})

	var createAndWaitForRunning = func() {
		By("Create ElasticsearchVersion: " + elasticsearchVersion.Name)
		err = f.CreateElasticsearchVersion(elasticsearchVersion)
		Expect(err).NotTo(HaveOccurred())

		By("Create Elasticsearch: " + elasticsearch.Name)
		err = f.CreateElasticsearch(elasticsearch)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running elasticsearch")
		f.EventuallyElasticsearchRunning(elasticsearch.ObjectMeta).Should(BeTrue())
	}

	var deleteTestResource = func() {
		if elasticsearch == nil {
			Skip("Skipping")
		}
		By("Delete elasticsearch: " + elasticsearch.Name)
		err = f.DeleteElasticsearch(elasticsearch.ObjectMeta)
		if err != nil {
			if kerr.IsNotFound(err) {
				// Elasticsearch was not created. Hence, rest of cleanup is not necessary.
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

		By("Wait for elasticsearch to be paused")
		f.EventuallyDormantDatabaseStatus(elasticsearch.ObjectMeta).Should(matcher.HavePaused())

		By("Set DormantDatabase Spec.WipeOut to true")
		_, err := f.PatchDormantDatabase(elasticsearch.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
			in.Spec.WipeOut = true
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Delete Dormant Database")
		err = f.DeleteDormantDatabase(elasticsearch.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for elasticsearch resources to be wipedOut")
		f.EventuallyWipedOut(elasticsearch.ObjectMeta).Should(Succeed())
	}

	AfterEach(func() {
		// Delete test resource
		deleteTestResource()

		for _, es := range garbageElasticsearch.Items {
			*elasticsearch = es
			// Delete test resource
			deleteTestResource()
		}

		if !skipSnapshotDataChecking {
			By("Check for snapshot data")
			f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
		}

		if secret != nil {
			f.DeleteSecret(secret.ObjectMeta)
		}

		err = f.DeleteElasticsearchVersion(elasticsearchVersion.ObjectMeta)
		if err != nil && !kerr.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("Test", func() {

		Context("General", func() {

			Context("-", func() {

				var shouldRunSuccessfully = func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					// Create Elasticsearch
					createAndWaitForRunning()

					By("Check for Elastic client")
					f.EventuallyElasticsearchClientReady(elasticsearch.ObjectMeta).Should(BeTrue())

					elasticClient, err := f.GetElasticClient(elasticsearch.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Creating new indices")
					err = elasticClient.CreateIndex(2)
					Expect(err).NotTo(HaveOccurred())

					By("Checking new indices")
					f.EventuallyElasticsearchIndicesCount(elasticClient).Should(Equal(3))

					elasticClient.Stop()

					By("Delete elasticsearch")
					err = f.DeleteElasticsearch(elasticsearch.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for elasticsearch to be paused")
					f.EventuallyDormantDatabaseStatus(elasticsearch.ObjectMeta).Should(matcher.HavePaused())

					// Create Elasticsearch object again to resume it
					By("Create Elasticsearch: " + elasticsearch.Name)
					err = f.CreateElasticsearch(elasticsearch)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(elasticsearch.ObjectMeta).Should(BeFalse())

					By("Wait for Running elasticsearch")
					f.EventuallyElasticsearchRunning(elasticsearch.ObjectMeta).Should(BeTrue())

					By("Check for Elastic client")
					f.EventuallyElasticsearchClientReady(elasticsearch.ObjectMeta).Should(BeTrue())

					elasticClient, err = f.GetElasticClient(elasticsearch.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Checking new indices")
					f.EventuallyElasticsearchIndicesCount(elasticClient).Should(Equal(3))
				}

				Context("with Default Resource", func() {

					It("should run successfully", shouldRunSuccessfully)

				})

				Context("Custom Resource", func() {
					BeforeEach(func() {
						elasticsearch.Spec.Resources = &core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("512Mi"),
							},
						}
					})

					It("should run successfully", shouldRunSuccessfully)

				})
			})

			Context("Dedicated elasticsearch", func() {
				BeforeEach(func() {
					elasticsearch = f.DedicatedElasticsearch()
				})

				var shouldRunSuccessfully = func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}
					// Create Elasticsearch
					createAndWaitForRunning()
				}

				Context("with Default Resource", func() {

					It("should run successfully", shouldRunSuccessfully)

				})

				Context("Custom Resource", func() {
					BeforeEach(func() {
						elasticsearch.Spec.Topology.Client.Resources = core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("128Mi"),
							},
						}
						elasticsearch.Spec.Topology.Master.Resources = core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("128Mi"),
							},
						}
						elasticsearch.Spec.Topology.Data.Resources = core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("128Mi"),
							},
						}
					})

					It("should run successfully", shouldRunSuccessfully)

				})

			})

		})

		Context("DoNotPause", func() {
			BeforeEach(func() {
				elasticsearch.Spec.DoNotPause = true
			})

			It("should work successfully", func() {
				// Create and wait for running Elasticsearch
				createAndWaitForRunning()

				By("Delete elasticsearch")
				err = f.DeleteElasticsearch(elasticsearch.ObjectMeta)
				Expect(err).Should(HaveOccurred())

				By("Elasticsearch is not paused. Check for elasticsearch")
				f.EventuallyElasticsearch(elasticsearch.ObjectMeta).Should(BeTrue())

				By("Check for Running elasticsearch")
				f.EventuallyElasticsearchRunning(elasticsearch.ObjectMeta).Should(BeTrue())

				By("Update elasticsearch to set DoNotPause=false")
				f.TryPatchElasticsearch(elasticsearch.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.DoNotPause = false
					return in
				})
			})
		})

		Context("Snapshot", func() {
			BeforeEach(func() {
				skipSnapshotDataChecking = false
				snapshot.Spec.DatabaseName = elasticsearch.Name
			})

			var shouldTakeSnapshot = func() {
				// Create and wait for running Elasticsearch
				createAndWaitForRunning()

				By("Create Secret")
				err := f.CreateSecret(secret)
				Expect(err).NotTo(HaveOccurred())

				By("Create Snapshot")
				err = f.CreateSnapshot(snapshot)
				Expect(err).NotTo(HaveOccurred())

				By("Check for Successed snapshot")
				f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

				if !skipSnapshotDataChecking {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
				}
			}

			Context("In Local", func() {
				BeforeEach(func() {
					skipSnapshotDataChecking = true
					secret = f.SecretForLocalBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Local = &store.LocalSpec{
						MountPath: "/repo",
						VolumeSource: core.VolumeSource{
							EmptyDir: &core.EmptyDirVolumeSource{},
						},
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In S3", func() {
				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &store.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In GCS", func() {
				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &store.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In Azure", func() {
				BeforeEach(func() {
					secret = f.SecretForAzureBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Azure = &store.AzureSpec{
						Container: os.Getenv(AZURE_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In Swift", func() {
				BeforeEach(func() {
					secret = f.SecretForSwiftBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Swift = &store.SwiftSpec{
						Container: os.Getenv(SWIFT_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})
		})

		Context("Initialize", func() {
			BeforeEach(func() {
				skipSnapshotDataChecking = false
				secret = f.SecretForS3Backend()
				snapshot.Spec.StorageSecretName = secret.Name
				snapshot.Spec.S3 = &store.S3Spec{
					Bucket: os.Getenv(S3_BUCKET_NAME),
				}
				snapshot.Spec.DatabaseName = elasticsearch.Name
			})

			It("should run successfully", func() {
				// Create and wait for running Elasticsearch
				createAndWaitForRunning()

				By("Create Secret")
				f.CreateSecret(secret)

				By("Check for Elastic client")
				f.EventuallyElasticsearchClientReady(elasticsearch.ObjectMeta).Should(BeTrue())

				elasticClient, err := f.GetElasticClient(elasticsearch.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Creating new indices")
				err = elasticClient.CreateIndex(2)
				Expect(err).NotTo(HaveOccurred())

				By("Checking new indices")
				f.EventuallyElasticsearchIndicesCount(elasticClient).Should(Equal(3))

				elasticClient.Stop()

				By("Create Snapshot")
				f.CreateSnapshot(snapshot)

				By("Check for Successed snapshot")
				f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

				By("Check for snapshot data")
				f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())

				oldElasticsearch, err := f.GetElasticsearch(elasticsearch.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				garbageElasticsearch.Items = append(garbageElasticsearch.Items, *oldElasticsearch)

				By("Create elasticsearch from snapshot")
				*elasticsearch = *f.CombinedElasticsearch()
				elasticsearch.Spec.Init = &api.InitSpec{
					SnapshotSource: &api.SnapshotSourceSpec{
						Namespace: snapshot.Namespace,
						Name:      snapshot.Name,
					},
				}

				// Create and wait for running Elasticsearch
				createAndWaitForRunning()

				By("Check for Elastic client")
				f.EventuallyElasticsearchClientReady(elasticsearch.ObjectMeta).Should(BeTrue())

				elasticClient, err = f.GetElasticClient(elasticsearch.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Checking indices")
				f.EventuallyElasticsearchIndicesCount(elasticClient).Should(Equal(3))
			})
		})

		Context("Resume", func() {
			var usedInitialized bool
			BeforeEach(func() {
				usedInitialized = false
			})

			var shouldResumeSuccessfully = func() {
				// Create and wait for running Elasticsearch
				createAndWaitForRunning()

				By("Delete elasticsearch")
				f.DeleteElasticsearch(elasticsearch.ObjectMeta)

				By("Wait for elasticsearch to be paused")
				f.EventuallyDormantDatabaseStatus(elasticsearch.ObjectMeta).Should(matcher.HavePaused())

				// Create Elasticsearch object again to resume it
				By("Create Elasticsearch: " + elasticsearch.Name)
				err = f.CreateElasticsearch(elasticsearch)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for DormantDatabase to be deleted")
				f.EventuallyDormantDatabase(elasticsearch.ObjectMeta).Should(BeFalse())

				By("Wait for Running elasticsearch")
				f.EventuallyElasticsearchRunning(elasticsearch.ObjectMeta).Should(BeTrue())

				es, err := f.GetElasticsearch(elasticsearch.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				*elasticsearch = *es
				if usedInitialized {
					_, ok := elasticsearch.Annotations[api.AnnotationInitialized]
					Expect(ok).Should(BeTrue())
				}
			}

			Context("-", func() {
				It("should resume DormantDatabase successfully", shouldResumeSuccessfully)
			})
		})

		Context("SnapshotScheduler", func() {
			AfterEach(func() {
				f.DeleteSecret(secret.ObjectMeta)
			})

			BeforeEach(func() {
				secret = f.SecretForLocalBackend()
			})

			Context("With Startup", func() {
				BeforeEach(func() {
					elasticsearch.Spec.BackupSchedule = &api.BackupScheduleSpec{
						CronExpression: "@every 1m",
						Backend: store.Backend{
							StorageSecretName: secret.Name,
							Local: &store.LocalSpec{
								MountPath: "/repo",
								VolumeSource: core.VolumeSource{
									EmptyDir: &core.EmptyDirVolumeSource{},
								},
							},
						},
					}
				})

				It("should run schedular successfully", func() {
					// Create and wait for running Elasticsearch
					createAndWaitForRunning()

					By("Create Secret")
					f.CreateSecret(secret)

					By("Count multiple Snapshot")
					f.EventuallySnapshotCount(elasticsearch.ObjectMeta).Should(matcher.MoreThan(3))
				})
			})

			Context("With Update", func() {
				It("should run schedular successfully", func() {
					// Create and wait for running Elasticsearch
					createAndWaitForRunning()

					By("Create Secret")
					f.CreateSecret(secret)

					By("Update elasticsearch")
					_, err = f.TryPatchElasticsearch(elasticsearch.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Local: &store.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										EmptyDir: &core.EmptyDirVolumeSource{},
									},
								},
							},
						}

						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Count multiple Snapshot")
					f.EventuallySnapshotCount(elasticsearch.ObjectMeta).Should(matcher.MoreThan(3))
				})
			})
		})

		Context("Environment Variables", func() {

			allowedEnvList := []core.EnvVar{
				{
					Name:  "CLUSTER_NAME",
					Value: "kubedb-es-e2e-cluster",
				},
				{
					Name:  "NUMBER_OF_MASTERS",
					Value: "1",
				},
				{
					Name:  "ES_JAVA_OPTS",
					Value: "-Xms256m -Xmx256m",
				},
				{
					Name:  "REPO_LOCATIONS",
					Value: "/backup",
				},
				{
					Name:  "MEMORY_LOCK",
					Value: "true",
				},
				{
					Name:  "HTTP_ENABLE",
					Value: "true",
				},
			}

			forbiddenEnvList := []core.EnvVar{
				{
					Name:  "NODE_NAME",
					Value: "kubedb-es-e2e-node",
				},
				{
					Name:  "NODE_MASTER",
					Value: "true",
				},
				{
					Name:  "NODE_DATA",
					Value: "true",
				},
			}

			var shouldRunSuccessfully = func() {
				if skipMessage != "" {
					Skip(skipMessage)
				}

				// Create Elasticsearch
				createAndWaitForRunning()

				By("Check for Elastic client")
				f.EventuallyElasticsearchClientReady(elasticsearch.ObjectMeta).Should(BeTrue())

				elasticClient, err := f.GetElasticClient(elasticsearch.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Creating new indices")
				err = elasticClient.CreateIndex(2)
				Expect(err).NotTo(HaveOccurred())

				By("Checking new indices")
				f.EventuallyElasticsearchIndicesCount(elasticClient).Should(Equal(3))

				elasticClient.Stop()

				By("Delete elasticsearch")
				err = f.DeleteElasticsearch(elasticsearch.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for elasticsearch to be paused")
				f.EventuallyDormantDatabaseStatus(elasticsearch.ObjectMeta).Should(matcher.HavePaused())

				// Create Elasticsearch object again to resume it
				By("Create Elasticsearch: " + elasticsearch.Name)
				err = f.CreateElasticsearch(elasticsearch)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for DormantDatabase to be deleted")
				f.EventuallyDormantDatabase(elasticsearch.ObjectMeta).Should(BeFalse())

				By("Wait for Running elasticsearch")
				f.EventuallyElasticsearchRunning(elasticsearch.ObjectMeta).Should(BeTrue())

				By("Check for Elastic client")
				f.EventuallyElasticsearchClientReady(elasticsearch.ObjectMeta).Should(BeTrue())

				elasticClient, err = f.GetElasticClient(elasticsearch.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Checking new indices")
				f.EventuallyElasticsearchIndicesCount(elasticClient).Should(Equal(3))
			}

			Context("With allowed Envs", func() {

				It("should run successfully with given envs.", func() {
					elasticsearch.Spec.PodTemplate.Spec.Env = allowedEnvList
					shouldRunSuccessfully()

					By("Checking pod started with given envs")
					pod, err := f.KubeClient().CoreV1().Pods(elasticsearch.Namespace).Get(elasticsearch.Name+"-0", metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())

					out, err := exec_util.ExecIntoPod(f.RestConfig(), pod, "env")
					Expect(err).NotTo(HaveOccurred())
					for _, env := range allowedEnvList {
						Expect(out).Should(ContainSubstring(env.Name + "=" + env.Value))
					}
				})

			})

			Context("With forbidden Envs", func() {

				It("should reject to create Elasticsearch CRD", func() {
					for _, env := range forbiddenEnvList {
						elasticsearch.Spec.PodTemplate.Spec.Env = []core.EnvVar{
							env,
						}

						By("Creating Elasticsearch with " + env.Name + " env var.")
						err := f.CreateElasticsearch(elasticsearch)
						Expect(err).To(HaveOccurred())
					}
				})

			})

			Context("Update Envs", func() {

				It("should reject to update Envs", func() {
					elasticsearch.Spec.PodTemplate.Spec.Env = allowedEnvList

					shouldRunSuccessfully()

					By("Updating Envs")
					_, _, err := util.PatchElasticsearch(f.ExtClient(), elasticsearch, func(in *api.Elasticsearch) *api.Elasticsearch {
						in.Spec.PodTemplate.Spec.Env = []core.EnvVar{
							{
								Name:  "CLUSTER_NAME",
								Value: "kubedb-es-e2e-cluster-patched",
							},
						}
						return in
					})
					Expect(err).To(HaveOccurred())
				})

			})
		})

		Context("Custom Configuration", func() {

			var userConfig *core.ConfigMap

			Context("With Topology", func() {
				BeforeEach(func() {
					elasticsearch = f.DedicatedElasticsearch()
					userConfig = f.GetCustomConfig()
				})

				AfterEach(func() {
					By("Deleting configMap: " + userConfig.Name)
					f.DeleteConfigMap(userConfig.ObjectMeta)
				})

				It("should use config provided in config files", func() {
					userConfig.Data = map[string]string{
						"common-config.yaml": f.GetCommonConfig(),
						"master-config.yaml": f.GetMasterConfig(),
						"client-config.yaml": f.GetClientConfig(),
						"data-config.yaml":   f.GetDataConfig(),
					}

					By("Creating configMap: " + userConfig.Name)
					err := f.CreateConfigMap(userConfig)
					Expect(err).NotTo(HaveOccurred())

					elasticsearch.Spec.ConfigSource = &core.VolumeSource{
						ConfigMap: &core.ConfigMapVolumeSource{
							LocalObjectReference: core.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}

					// Create Elasticsearch
					createAndWaitForRunning()

					By("Check for Elastic client")
					f.EventuallyElasticsearchClientReady(elasticsearch.ObjectMeta).Should(BeTrue())

					elasticClient, err := f.GetElasticClient(elasticsearch.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Reading Nodes information")
					settings, err := elasticClient.GetAllNodesInfo()
					Expect(err).NotTo(HaveOccurred())

					By("Checking nodes are using provided config")
					Expect(f.IsUsingProvidedConfig(settings)).Should(BeTrue())

					elasticClient.Stop()
				})

			})

			Context("Without Topology", func() {
				BeforeEach(func() {
					userConfig = f.GetCustomConfig()
				})

				AfterEach(func() {
					By("Deleting configMap: " + userConfig.Name)
					f.DeleteConfigMap(userConfig.ObjectMeta)
				})

				It("should use config provided in config files", func() {
					userConfig.Data = map[string]string{
						"common-config.yaml": f.GetCommonConfig(),
						"master-config.yaml": f.GetMasterConfig(),
						"client-config.yaml": f.GetClientConfig(),
						"data-config.yaml":   f.GetDataConfig(),
					}

					By("Creating configMap: " + userConfig.Name)
					err := f.CreateConfigMap(userConfig)
					Expect(err).NotTo(HaveOccurred())

					elasticsearch.Spec.ConfigSource = &core.VolumeSource{
						ConfigMap: &core.ConfigMapVolumeSource{
							LocalObjectReference: core.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}

					// Create Elasticsearch
					createAndWaitForRunning()

					By("Check for Elastic client")
					f.EventuallyElasticsearchClientReady(elasticsearch.ObjectMeta).Should(BeTrue())

					elasticClient, err := f.GetElasticClient(elasticsearch.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Reading Nodes information")
					settings, err := elasticClient.GetAllNodesInfo()
					Expect(err).NotTo(HaveOccurred())

					By("Checking nodes are using provided config")
					Expect(f.IsUsingProvidedConfig(settings)).Should(BeTrue())

					elasticClient.Stop()
				})

			})
		})
	})
})

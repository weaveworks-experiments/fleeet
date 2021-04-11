/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
	fleetv1 "github.com/squaremo/fleeet/control/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
)

const (
	timeout  = 5 * time.Second
	interval = time.Second
)

var _ = Describe("remote assemblages", func() {

	var (
		manager             ctrl.Manager
		stopManager         func()
		managerDone         chan struct{}
		downstreamK8sClient client.Client
		downstreamEnv       *envtest.Environment
		cluster             *clusterv1.Cluster
		clusterSecret       *corev1.Secret
	)

	BeforeEach(func() {
		// downstream "cluster" test env
		downstreamEnv, cluster, clusterSecret, downstreamK8sClient = makeDownstreamEnv()

		By("starting a controller manager")
		var err error
		manager, err = ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme.Scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		remoteReconciler := &RemoteAssemblageReconciler{
			Client: manager.GetClient(),
			Log:    ctrl.Log.WithName("controllers").WithName("RemoteAssemblage"),
			Scheme: manager.GetScheme(),
		}
		Expect(remoteReconciler.SetupWithManager(manager)).To(Succeed())

		var ctx context.Context
		ctx, stopManager = context.WithCancel(signalHandler)
		managerDone = make(chan struct{})
		go func() {
			defer GinkgoRecover()
			Expect(manager.Start(ctx)).To(Succeed())
			close(managerDone)
		}()
	})

	AfterEach(func() {
		stopManager()

		By("removing cluster records")
		Expect(k8sClient.Delete(context.Background(), cluster)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), clusterSecret)).To(Succeed())

		By("tearing down the test environment")
		err := downstreamEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
		<-managerDone
	})

	Context("proxying", func() {
		It("makes a sync in the downstream", func() {
			// make a proxy object in the mgmt cluster
			proxy := fleetv1.RemoteAssemblage{
				Spec: fleetv1.RemoteAssemblageSpec{
					KubeconfigRef: fleetv1.LocalKubeconfigReference{Name: clusterSecret.Name},
					Assemblage: asmv1.AssemblageSpec{
						Syncs: []syncapi.NamedSync{
							{
								Name: "app",
								Sync: syncapi.Sync{
									Source: syncapi.SourceSpec{
										Git: &syncapi.GitSource{
											URL:     "https://github.com/cuttlefacts/cuttlefacts-app",
											Version: syncapi.GitVersion{Tag: "v0.3.0"},
										},
									},
								},
							},
						},
					},
				},
			}
			proxy.Name = "test-proxy-sync"
			proxy.Namespace = "default"
			Expect(k8sClient.Create(context.Background(), &proxy)).To(Succeed())

			var asm asmv1.Assemblage
			Eventually(func() bool {
				err := downstreamK8sClient.Get(context.Background(), types.NamespacedName{
					Name:      "test-proxy-sync",
					Namespace: "default",
				}, &asm)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(asm.Spec.Syncs).To(Equal(proxy.Spec.Assemblage.Syncs))
		})
	})
})

// This has funcs for creating a downstream cluster, useful for
// testing remote/proxy syncs.

// makeDownstreamEnv constructs an envtest Environment, and creates a
// Cluster object in the regular env pointing to it. It returns the
// downstream environment, the cluster object, the secret with
// connection creds, and a client for the downstream environment. The
// downstream environment has the CRDs already installed.
func makeDownstreamEnv() (*envtest.Environment, *clusterv1.Cluster, *corev1.Secret, client.Client) {
	By("bootstrapping downstream cluster environment")
	downstreamEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "assemblage", "config", "crd", "bases")},
	}

	downstreamCfg, err := downstreamEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(downstreamCfg).ToNot(BeNil())

	// this is for the test to verify things are correct in the
	// downstream cluster; therefore we do want to be able to
	// understand the various CRDs.
	downstreamK8sClient, err := client.New(downstreamCfg, client.Options{Scheme: scheme.Scheme})
	Expect(downstreamK8sClient).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	// TODO details of the cluster
	cluster := &clusterv1.Cluster{}
	cluster.Name = "downstream"
	cluster.Namespace = "default"
	Expect(k8sClient.Create(context.Background(), cluster)).To(Succeed())

	cluster.Status.ControlPlaneReady = true                         // ) i.e., all ready
	cluster.Status.InfrastructureReady = true                       // )
	cluster.Status.SetTypedPhase(clusterv1.ClusterPhaseProvisioned) // )
	Expect(k8sClient.Status().Update(context.Background(), cluster)).To(Succeed())

	// For creating a secret:
	// https://github.com/kubernetes-sigs/cluster-api/blob/e5b02bdbce6c32b4dc062e9e1f14f8ccd16e8952/util/kubeconfig/kubeconfig.go#L109
	config := kubeconfigFromEndpoint("downstream", downstreamEnv.ControlPlane.APIURL().String())
	clusterSecretData, err := clientcmd.Write(*config)
	Expect(err).ToNot(HaveOccurred())

	clusterSecret := kubeconfig.GenerateSecret(cluster, clusterSecretData)
	Expect(k8sClient.Create(context.Background(), clusterSecret)).To(Succeed())

	return downstreamEnv, cluster, clusterSecret, downstreamK8sClient
}

func kubeconfigFromEndpoint(clusterName, endpoint string) *api.Config {
	username := fmt.Sprintf("%s-admin", clusterName)
	contextName := fmt.Sprintf("%s@%s", username, clusterName)
	return &api.Config{
		Clusters: map[string]*api.Cluster{
			clusterName: {
				Server: endpoint,
			},
		},
		Contexts: map[string]*api.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: username,
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			username: {},
		},
		CurrentContext: contextName,
	}
}

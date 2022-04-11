/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"
	//"fmt"
	//	"path/filepath"
	//	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	// corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	// ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	//kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	//meta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

	fleetv1 "github.com/squaremo/fleeet/module/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
)

var _ = Describe("bootstrap module type", func() {

	It("rejects invalid specs", func() {
		for _, mod := range []fleetv1.BootstrapModule{
			{}, // nothing specified!
			{
				Spec: fleetv1.BootstrapModuleSpec{
					// no sync given
				},
			},
			{
				Spec: fleetv1.BootstrapModuleSpec{
					Sync: syncapi.Sync{
						// source is required
					},
				},
			},
		} {
			mod.Name = randString(5)
			Expect(k8sClient.Create(context.TODO(), &mod)).ToNot(Succeed())
		}
	})

	It("can create a minimal bootstrap module", func() {
		bootmod := fleetv1.BootstrapModule{
			Spec: fleetv1.BootstrapModuleSpec{
				Sync: syncapi.Sync{
					Source: syncapi.SourceSpec{
						Git: &syncapi.GitSource{
							URL: "https://github.com/cuttlefacts/cuttlefacts-app",
							Version: syncapi.GitVersion{
								Tag: "1.0.0",
							},
						},
					},
					// package is not required
				},
			},
		}
		bootmod.Name = "boot"
		bootmod.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), &bootmod)).To(Succeed())
	})
})

var _ = Describe("bootstrap module controller", func() {

	var (
		manager     ctrl.Manager
		stopManager func()
		managerDone chan struct{}
	)

	BeforeEach(func() {
		By("starting a controller manager")
		var err error
		manager, err = ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme.Scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		bootmodReconciler := &BootstrapModuleReconciler{
			Client: manager.GetClient(),
			Log:    ctrl.Log.WithName("controllers").WithName("BootstrapModule"),
			Scheme: manager.GetScheme(),
		}
		Expect(bootmodReconciler.SetupWithManager(manager)).To(Succeed())

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
		<-managerDone
	})

	var (
		namespace corev1.Namespace
		mod       fleetv1.BootstrapModule
	)

	BeforeEach(func() {
		namespace = corev1.Namespace{}
		namespace.Name = randString(5)
		Expect(k8sClient.Create(context.TODO(), &namespace)).To(Succeed())

		mod = fleetv1.BootstrapModule{
			Spec: fleetv1.BootstrapModuleSpec{
				Selector: &metav1.LabelSelector{}, // all clusters
				ControlPlaneBindings: []syncapi.Binding{
					{
						Name: "cluster.name",
						BindingSource: syncapi.BindingSource{
							StringValue: &syncapi.StringValue{
								Value: "$(CLUSTER_NAME)",
							},
						},
					},
				},
				Sync: syncapi.Sync{
					Source: syncapi.SourceSpec{
						Git: &syncapi.GitSource{
							URL: "https://github.com/cuttlefacts/cuttlefacts-app",
							Version: syncapi.GitVersion{
								Tag: "v0.1.0",
							},
						},
					},
					Package: &syncapi.PackageSpec{
						Kustomize: &syncapi.KustomizeSpec{
							Path: "./deploy",
							Substitute: map[string]string{
								"cluster.name": "$(cluster.name)",
							},
						},
					},
				},
			},
		}
		mod.Namespace = namespace.Name
		mod.Name = randString(5)
		Expect(k8sClient.Create(context.TODO(), &mod)).To(Succeed())
	})

	var (
		clusters map[string]*clusterv1.Cluster
	)

	BeforeEach(func() {
		// Create clusters so I can check e.g., that there's a RemoteAssemblage per cluster,
		// targeting the cluster.
		clusters = make(map[string]*clusterv1.Cluster)
		for i := 0; i < 3; i++ {
			cluster := &clusterv1.Cluster{}
			cluster.Name = "cluster-" + randString(5)
			cluster.Namespace = namespace.Name
			clusters[cluster.Name] = cluster
			Expect(k8sClient.Create(context.TODO(), cluster)).To(Succeed())
			cluster.Status.ControlPlaneReady = true
			Expect(k8sClient.Status().Update(context.Background(), cluster)).To(Succeed())
		}

	})

	It("creates a source per module", func() {
		// Check there's a GitRepository created for the module
		var src sourcev1.GitRepository
		Eventually(func() bool {
			err := k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: mod.Namespace,
				Name:      mod.Name,
			}, &src)
			return err == nil
		}, "5s", "1s").Should(BeTrue())

		Expect(metav1.IsControlledBy(&src, &mod)).To(BeTrue())
	})

	It("creates a RemoteAssemblage per cluster", func() {
		expectedClusters := make(map[string]*clusterv1.Cluster)
		for n, v := range clusters {
			expectedClusters[n] = v
		}

		var asms fleetv1.RemoteAssemblageList
		Eventually(func() bool {
			if err := k8sClient.List(context.TODO(), &asms, client.InNamespace(namespace.Name)); err != nil {
				return false
			}
			return len(asms.Items) >= len(clusters)
		}, "5s", "1s").Should(BeTrue())
		Expect(len(asms.Items)).To(Equal(len(clusters)))

		for _, asm := range asms.Items {
			clusterName := asm.Spec.KubeconfigRef.Name[:len(asm.Spec.KubeconfigRef.Name)-len("-kubeconfig")]
			Expect(expectedClusters).To(HaveKey(clusterName))
			delete(expectedClusters, clusterName)
			// check properties of assemblage: has the expected syncs, owner
			// TODO owner
			syncs := asm.Spec.Syncs
			Expect(len(syncs)).To(Equal(1))
			sync := syncs[0]
			switch sync.Name {
			case mod.Name:
				Expect(sync.SourceRef.Name).ToNot(BeEmpty())
			default:
				Fail("unexpected sync in assemblage spec: " + sync.Name)
			}
		}
		Expect(expectedClusters).To(BeEmpty())
	})

})

/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"
	//	"fmt"
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

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

	fleetv1 "github.com/squaremo/fleeet/control/api/v1alpha1"
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

	It("expands bootstrap modules", func() {
		namespace := corev1.Namespace{}
		namespace.Name = randString(5)
		Expect(k8sClient.Create(context.TODO(), &namespace)).To(Succeed())

		mod := fleetv1.BootstrapModule{
			Spec: fleetv1.BootstrapModuleSpec{
				Selector: &metav1.LabelSelector{}, // all clusters
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
						},
					},
				},
			},
		}
		mod.Namespace = namespace.Name
		mod.Name = randString(5)
		Expect(k8sClient.Create(context.TODO(), &mod)).To(Succeed())

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

		// Create clusters and check there's a Kustomization per
		// cluster, targeting the cluster.
		clusters := make([]clusterv1.Cluster, 3)
		for i := range clusters {
			clusters[i].Name = "cluster-" + randString(5)
			clusters[i].Namespace = namespace.Name
			Expect(k8sClient.Create(context.TODO(), &clusters[i])).To(Succeed())
		}

		var kustoms kustomv1.KustomizationList
		Eventually(func() bool {
			if err := k8sClient.List(context.TODO(), &kustoms, client.InNamespace(namespace.Name)); err != nil {
				return false
			}
			return len(kustoms.Items) >= len(clusters)
		}, "5s", "1s").Should(BeTrue())
		Expect(len(kustoms.Items)).To(Equal(len(clusters)))

		// TODO check each one corresponds to a cluster, targets the
		// cluster, and has the bootstrapmodule sync
	})
})

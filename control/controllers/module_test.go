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
	//	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime"
	//	"sigs.k8s.io/controller-runtime/pkg/client"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
	fleetv1 "github.com/squaremo/fleeet/control/api/v1alpha1"
)

// makeSync is a convenience for testing, which creates a sync with
// the given name, git URL, and version tag.
func makeSync(name, url, tag string) asmv1.Sync {
	return asmv1.Sync{
		Name: name,
		Source: asmv1.SourceSpec{
			Git: &asmv1.GitSource{
				URL: url,
				Version: asmv1.GitVersion{
					Tag: tag,
				},
			},
		},
		// leave package to default
	}
}

var _ = Describe("modules", func() {
	var (
		manager     ctrl.Manager
		stopManager func()
	)

	BeforeEach(func() {
		By("starting a controller manager")
		var err error
		manager, err = ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme.Scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		moduleReconciler := &ModuleReconciler{
			Client: manager.GetClient(),
			Log:    ctrl.Log.WithName("controllers").WithName("Module"),
			Scheme: manager.GetScheme(),
		}
		Expect(moduleReconciler.SetupWithManager(manager)).To(Succeed())

		var ctx context.Context
		ctx, stopManager = context.WithCancel(ctrl.SetupSignalHandler())
		go func() {
			defer GinkgoRecover()
			Expect(manager.Start(ctx)).To(Succeed())
		}()
	})

	AfterEach(func() {
		stopManager()
	})

	FContext("compiles remote assemblages", func() {

		var (
			cluster *clusterv1.Cluster
		)

		BeforeEach(func() {
			// TODO details of the cluster
			cluster = &clusterv1.Cluster{}
			cluster.Name = "downstream"
			cluster.Namespace = "default"
			Expect(k8sClient.Create(context.Background(), cluster)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), cluster)).To(Succeed())
		})

		Context("matching all clusters", func() {
			It("creates a remote assemblage", func() {
				module := &fleetv1.Module{
					Spec: fleetv1.ModuleSpec{
						Selector: &metav1.LabelSelector{}, // all clusters
						Sync:     makeSync("app", "https://github.com/cuttlefacts/app", "v0.3.4"),
					},
				}
				module.Name = "testmod"
				module.Namespace = "default"
				Expect(k8sClient.Create(context.Background(), module)).To(Succeed())

				var asm fleetv1.RemoteAssemblage
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Namespace: module.Namespace,
						Name:      cluster.Name, // FIXME what is the name?
					}, &asm)
					return err == nil
				}, "5s", "1s").Should(BeTrue())

				// TODO now look in the assemblage for the module and its version
			})
		})
	})
})

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
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		managerDone chan struct{}
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

	Context("compiles remote assemblages", func() {

		var (
			clusters = []string{
				"cluster-1",
				"cluster-2",
				"cluster-3",
			}
		)

		BeforeEach(func() {
			for _, name := range clusters {
				cluster := &clusterv1.Cluster{}
				cluster.Name = name
				cluster.Namespace = "default"
				Expect(k8sClient.Create(context.Background(), cluster)).To(Succeed())
			}
			// TODO details of the cluster
		})

		AfterEach(func() {
			Expect(k8sClient.DeleteAllOf(context.TODO(), &clusterv1.Cluster{}, client.InNamespace("default"))).To(Succeed())
			Expect(k8sClient.DeleteAllOf(context.TODO(), &fleetv1.Module{}, client.InNamespace("default"))).To(Succeed())
			Expect(k8sClient.DeleteAllOf(context.TODO(), &fleetv1.RemoteAssemblage{}, client.InNamespace("default"))).To(Succeed())
		})

		Context("matching clusters", func() {
			It("creates remote assemblages with matching modules", func() {
				// Strategy: create the non-matching module first, then the matching module, and
				// wait until assemblages are created to signify that the second module has been
				// processed. Possible flaw: it's also a correct implementation for assemblages
				// to be created in response to the first module, or even in response to the
				// clusters existing.
				nomatchModule := &fleetv1.Module{
					Spec: fleetv1.ModuleSpec{
						// leave the selector out to indicate "match nothing"
						Sync: makeSync("app", "https://github.com/cuttlefacts/cuttlefacts-platform", "v0.1.2"),
					},
				}
				nomatchModule.Name = "nomatch"
				nomatchModule.Namespace = "default"
				Expect(k8sClient.Create(context.Background(), nomatchModule)).To(Succeed())

				matchModule := &fleetv1.Module{
					Spec: fleetv1.ModuleSpec{
						Selector: &metav1.LabelSelector{}, // all clusters
						Sync:     makeSync("app", "https://github.com/cuttlefacts/app", "v0.3.4"),
					},
				}
				matchModule.Name = "matches"
				matchModule.Namespace = "default"
				Expect(k8sClient.Create(context.Background(), matchModule)).To(Succeed())

				var asms fleetv1.RemoteAssemblageList
				Eventually(func() bool {
					err := k8sClient.List(context.TODO(), &asms, client.InNamespace("default"))
					return err == nil && len(asms.Items) == len(clusters)
				}, "5s", "1s").Should(BeTrue())

				for _, asm := range asms.Items {
					Expect(asm.Spec.Assemblage.Syncs).To(ContainElement(matchModule.Spec.Sync))
					Expect(asm.Spec.Assemblage.Syncs).NotTo(ContainElement(nomatchModule.Spec.Sync))
				}

				// add a cluster and check that it gets matched
				newCluster := clusterv1.Cluster{}
				newCluster.Name = "newcluster"
				newCluster.Namespace = "default"
				Expect(k8sClient.Create(context.TODO(), &newCluster)).To(Succeed())

				var newAsm fleetv1.RemoteAssemblage
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Namespace: "default",
						Name:      newCluster.Name,
					}, &newAsm)
					return err == nil
				}, "5s", "1s").Should(BeTrue())
				Expect(newAsm.Spec.Assemblage.Syncs).To(ContainElement(matchModule.Spec.Sync))
				Expect(newAsm.Spec.Assemblage.Syncs).NotTo(ContainElement(nomatchModule.Spec.Sync))
			})
		})
	})
})

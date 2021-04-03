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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
	fleetv1 "github.com/squaremo/fleeet/control/api/v1alpha1"
)

// makeSync is a convenience for testing, which creates a sync with
// the given name, git URL, and version tag.
func makeSync(url, tag string) asmv1.Sync {
	return asmv1.Sync{
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
			namespace *corev1.Namespace
			clusters  = []string{
				"cluster-1",
				"cluster-2",
				"cluster-3",
			}
		)

		BeforeEach(func() {
			namespace = &corev1.Namespace{}
			namespace.Name = "ns-" + randString(5)
			Expect(k8sClient.Create(context.TODO(), namespace)).To(Succeed())

			for _, name := range clusters {
				cluster := &clusterv1.Cluster{}
				cluster.Name = name
				cluster.Namespace = namespace.Name
				Expect(k8sClient.Create(context.Background(), cluster)).To(Succeed())
			}
			// TODO details of the cluster
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), namespace)).To(Succeed())
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
						Sync: makeSync("https://github.com/cuttlefacts/cuttlefacts-platform", "v0.1.2"),
					},
				}
				nomatchModule.Name = "nomatch"
				nomatchModule.Namespace = namespace.Name
				Expect(k8sClient.Create(context.Background(), nomatchModule)).To(Succeed())

				matchModule := &fleetv1.Module{
					Spec: fleetv1.ModuleSpec{
						Selector: &metav1.LabelSelector{}, // all clusters
						Sync:     makeSync("https://github.com/cuttlefacts/app", "v0.3.4"),
					},
				}
				matchModule.Name = "matches"
				matchModule.Namespace = namespace.Name
				Expect(k8sClient.Create(context.Background(), matchModule)).To(Succeed())

				var asms fleetv1.RemoteAssemblageList
				Eventually(func() bool {
					err := k8sClient.List(context.TODO(), &asms, client.InNamespace(namespace.Name))
					return err == nil && len(asms.Items) == len(clusters)
				}, "5s", "1s").Should(BeTrue())

				for _, asm := range asms.Items {
					Expect(asm.Spec.Assemblage.Syncs).To(ContainElement(asmv1.NamedSync{
						Name: "matches",
						Sync: matchModule.Spec.Sync,
					}))
					Expect(asm.Spec.Assemblage.Syncs).NotTo(ContainElement(asmv1.NamedSync{
						Name: "nomatch",
						Sync: nomatchModule.Spec.Sync,
					}))
				}

				// add a cluster and check that it gets matched
				newCluster := clusterv1.Cluster{}
				newCluster.Name = "newcluster"
				newCluster.Namespace = namespace.Name
				Expect(k8sClient.Create(context.TODO(), &newCluster)).To(Succeed())

				var newAsm fleetv1.RemoteAssemblage
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Namespace: namespace.Name,
						Name:      newCluster.Name,
					}, &newAsm)
					return err == nil
				}, "5s", "1s").Should(BeTrue())
				Expect(newAsm.Spec.Assemblage.Syncs).To(ContainElement(asmv1.NamedSync{
					Name: matchModule.Name,
					Sync: matchModule.Spec.Sync,
				}))
				Expect(newAsm.Spec.Assemblage.Syncs).NotTo(ContainElement(asmv1.NamedSync{
					Name: nomatchModule.Name,
					Sync: nomatchModule.Spec.Sync,
				}))
			})
		})

		Context("module updates", func() {
			It("updates remote assemblages with new version", func() {
				module := &fleetv1.Module{
					Spec: fleetv1.ModuleSpec{
						Selector: &metav1.LabelSelector{}, // all clusters
						Sync:     makeSync("https://github.com/cuttlefacts/app", "v0.3.4"),
					},
				}
				module.Name = "matches"
				module.Namespace = namespace.Name
				Expect(k8sClient.Create(context.TODO(), module)).To(Succeed())

				var asms fleetv1.RemoteAssemblageList
				Eventually(func() bool {
					err := k8sClient.List(context.TODO(), &asms, client.InNamespace(namespace.Name))
					return err == nil && len(asms.Items) == len(clusters)
				}, "5s", "1s").Should(BeTrue())
				for _, asm := range asms.Items {
					Expect(asm.Spec.Assemblage.Syncs).To(ContainElement(asmv1.NamedSync{
						Name: module.Name,
						Sync: module.Spec.Sync,
					}))
				}

				newTag := "v0.3.5"
				_, err := ctrlutil.CreateOrPatch(context.TODO(), k8sClient, module, func() error {
					module.Spec.Sync.Source.Git.Version.Tag = newTag
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() bool {
					err := k8sClient.List(context.TODO(), &asms, client.InNamespace(namespace.Name))
					if err != nil {
						return false
					}
					for _, asm := range asms.Items {
						if asm.Spec.Assemblage.Syncs[0].Source.Git.Version.Tag != newTag {
							return false
						}
					}
					return true
				}, "5s", "1s").Should(BeTrue())
			})
		})
	})

	Context("module status", func() {
		var (
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			namespace = &corev1.Namespace{}
			namespace.Name = "ns-" + randString(5)
			Expect(k8sClient.Create(context.TODO(), namespace)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), namespace)).To(Succeed())
		})

		It("reports aggregate stats in module status", func() {
			cluster := clusterv1.Cluster{}
			cluster.Namespace = namespace.Name
			cluster.Name = "clus-" + randString(5)
			Expect(k8sClient.Create(context.TODO(), &cluster)).To(Succeed())

			module := fleetv1.Module{
				Spec: fleetv1.ModuleSpec{
					Selector: &metav1.LabelSelector{}, // match all
					Sync:     makeSync("https://github.com/cuttlefacts/app", "v1.1.0"),
				},
			}
			module.Namespace = namespace.Name
			module.Name = "mod-" + randString(5)
			Expect(k8sClient.Create(context.TODO(), &module)).To(Succeed())

			var asms fleetv1.RemoteAssemblageList
			Eventually(func() bool {
				err := k8sClient.List(context.TODO(), &asms, client.InNamespace(namespace.Name))
				return err == nil && len(asms.Items) > 0
			}, "5s", "1s").Should(BeTrue())

			asm := asms.Items[0]
			Expect(asm.Spec.Assemblage.Syncs).To(ContainElement(asmv1.NamedSync{
				Name: module.Name,
				Sync: module.Spec.Sync,
			}))

			// All that is as expected. Now, give the assemblage a status,
			// and make sure it gets back to the module.
			syncs := asm.Spec.Assemblage.Syncs
			for _, s := range syncs {
				asm.Status.Syncs = append(asm.Status.Syncs, asmv1.SyncStatus{
					Sync:  s,
					State: asmv1.StateSucceeded,
				})
			}
			Expect(k8sClient.Status().Update(context.TODO(), &asm)).To(Succeed())

			var m fleetv1.Module
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: module.Namespace,
					Name:      module.Name,
				}, &m)
				if err != nil {
					return false
				}
				return m.Status.Summary != nil && m.Status.Summary.Succeeded > 0
			}, "5s", "1s").Should(BeTrue())
			Expect(m.Status.Summary.Total).To(Equal(1))
			Expect(m.Status.Summary.Succeeded).To(Equal(1))
		})
	})
})

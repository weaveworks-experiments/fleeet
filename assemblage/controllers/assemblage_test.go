/*
Copyright 2021 Michael Bridgen
*/

package controllers

import (
	"context"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
)

func randomStr(prefix string) string {
	const randomStrCount = 5
	var letterRunes []rune = []rune("abcdefghijklmnopqrstuvwxyz1234567890")
	b := make([]rune, randomStrCount)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return prefix + string(b)
}

var _ = Describe("assemblage controller", func() {
	var manager ctrl.Manager
	var stopManager func()
	var namespace *corev1.Namespace

	BeforeEach(func() {
		var err error
		manager, err = ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme.Scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		assemblageReconciler := &AssemblageReconciler{
			Client: manager.GetClient(),
			Scheme: scheme.Scheme,
			Log:    ctrl.Log.WithName("controllers").WithName("Assemblage"),
		}
		Expect(assemblageReconciler.SetupWithManager(manager)).To(Succeed())

		namespace = &corev1.Namespace{}
		namespace.Name = randomStr("test-ns-")
		Expect(k8sClient.Create(context.Background(), namespace)).To(Succeed())

		var ctx context.Context
		ctx, stopManager = context.WithCancel(context.Background())
		go func() {
			defer GinkgoRecover()
			err = manager.Start(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
	})

	AfterEach(func() {
		stopManager()
		Expect(k8sClient.Delete(context.Background(), namespace)).To(Succeed())
	})

	It("can be created", func() {
		asm := asmv1.Assemblage{
			Spec: asmv1.AssemblageSpec{
				Syncs: []syncapi.NamedSync{},
			},
		}
		asm.Name = "asm"
		asm.Namespace = namespace.Name

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		Expect(k8sClient.Create(ctx, &asm)).To(Succeed())
	})

	It("creates GOTK objects", func() {
		asm := asmv1.Assemblage{
			Spec: asmv1.AssemblageSpec{
				Syncs: []syncapi.NamedSync{
					{
						Name: "app",
						Sync: syncapi.Sync{
							Source: syncapi.SourceSpec{
								Git: &syncapi.GitSource{
									URL: "https://github.com/cuttlefacts-app",
									Version: syncapi.GitVersion{
										Revision: "bd6ef78",
									},
								},
							},
							Package: &syncapi.PackageSpec{
								Kustomize: &syncapi.KustomizeSpec{
									Path: "deploy",
								},
							},
						},
					},
				},
			},
		}
		asm.Name = randomStr("asm")
		asm.Namespace = namespace.Name

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		Expect(k8sClient.Create(ctx, &asm)).To(Succeed())

		// eventually we should see a git repository source and
		// kustomization created in the same namespace.
		expectedGitName := types.NamespacedName{
			Name:      asm.Name + "-0",
			Namespace: asm.Namespace,
		}
		Eventually(func() bool {
			var source sourcev1.GitRepository
			if err := k8sClient.Get(context.Background(), expectedGitName, &source); err != nil {
				return false
			}
			return source.Name == expectedGitName.Name
		}, "5s", "1s").Should(BeTrue())

		expectedKustomizationName := types.NamespacedName{
			Name:      asm.Name + "-0",
			Namespace: asm.Namespace,
		}
		var kustom kustomv1.Kustomization
		Eventually(func() bool {
			if err := k8sClient.Get(context.Background(), expectedKustomizationName, &kustom); err != nil {
				return false
			}
			return kustom.Name == expectedGitName.Name
		}, "5s", "1s").Should(BeTrue())
		Expect(kustom.Spec.Path).To(Equal(asm.Spec.Syncs[0].Package.Kustomize.Path))
	})

	Context("bindings", func() {

		var (
			asm          asmv1.Assemblage
			bindingValue string
		)

		BeforeEach(func() {
			bindingValue = randomStr("cuttlefacts")
			asm = asmv1.Assemblage{
				Spec: asmv1.AssemblageSpec{
					Bindings: []syncapi.Binding{
						{
							Name: "APP_NAME",
							BindingSource: syncapi.BindingSource{
								Value: &bindingValue,
							},
						},
					},
					Syncs: []syncapi.NamedSync{
						{
							Name: "app",
							Sync: syncapi.Sync{
								Source: syncapi.SourceSpec{
									Git: &syncapi.GitSource{
										URL: "https://github.com/cuttlefacts-app",
										Version: syncapi.GitVersion{
											Revision: "bd6ef78",
										},
									},
								},
								Package: &syncapi.PackageSpec{
									Kustomize: &syncapi.KustomizeSpec{
										Path: "deploy",
										Substitute: map[string]string{
											"APP_NAME": "foo $(APP_NAME)",
										},
									},
								},
							},
						},
					},
				},
			}
			asm.Name = randomStr("asm")
			asm.Namespace = namespace.Name
			Expect(k8sClient.Create(context.TODO(), &asm)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), &asm)).To(Succeed())
		})

		It("adds the substitution to the kustomization", func() {
			expectedKustomizationName := types.NamespacedName{
				Name:      asm.Name + "-0",
				Namespace: asm.Namespace,
			}
			var kustom kustomv1.Kustomization
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), expectedKustomizationName, &kustom); err != nil {
					return false
				}
				return kustom.Name == expectedKustomizationName.Name
			}, "5s", "1s").Should(BeTrue())
			Expect(kustom.Spec.Path).To(Equal(asm.Spec.Syncs[0].Package.Kustomize.Path))
			Expect(kustom.Spec.PostBuild).ToNot(BeNil())
			postbuild := kustom.Spec.PostBuild
			Expect(postbuild.Substitute).To(Equal(map[string]string{
				"APP_NAME": "foo " + bindingValue,
			}))
		})
	})
})

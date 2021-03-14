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

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	fleetv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
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
		asm := fleetv1.Assemblage{
			Spec: fleetv1.AssemblageSpec{
				Syncs: []fleetv1.Sync{},
			},
		}
		asm.Name = "asm"
		asm.Namespace = namespace.Name

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		Expect(k8sClient.Create(ctx, &asm)).To(Succeed())
	})

	It("creates GOTK objects", func() {
		asm := fleetv1.Assemblage{
			Spec: fleetv1.AssemblageSpec{
				Syncs: []fleetv1.Sync{
					{
						Source: fleetv1.SourceSpec{
							Git: &fleetv1.GitSource{
								URL: "https://github.com/cuttlefacts-app",
								Version: fleetv1.GitVersion{
									Revision: "bd6ef78",
								},
							},
						},
						Package: &fleetv1.PackageSpec{
							Kustomize: &fleetv1.KustomizeSpec{
								Path: "deploy",
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
	})
})

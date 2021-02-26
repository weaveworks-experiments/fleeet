/*
Copyright 2021 Michael Bridgen
*/

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	//	fleetv1alpha1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
)

var _ = Describe("assemblage controller", func() {
	var manager ctrl.Manager
	var stopManager func()

	BeforeEach(func() {
		var err error
		manager, err = ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme.Scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		assemblageReconciler := &AssemblageReconciler{
			Client: manager.GetClient(),
			Scheme: scheme.Scheme,
		}
		Expect(assemblageReconciler.SetupWithManager(manager)).To(Succeed())

		var ctx context.Context
		ctx, stopManager = context.WithCancel(context.Background())
		go func() {
			err = manager.Start(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
	})

	AfterEach(func() {
		stopManager()
	})
})

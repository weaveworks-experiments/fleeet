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
	// corev1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/apimachinery/pkg/types"
	// "k8s.io/client-go/kubernetes/scheme"
	// clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	// ctrl "sigs.k8s.io/controller-runtime"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	// "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	// ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fleetv1 "github.com/squaremo/fleeet/control/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
)

var _ = Describe("bootstrap modules", func() {

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

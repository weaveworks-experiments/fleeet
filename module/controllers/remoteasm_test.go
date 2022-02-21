/*
Copyright 2021, 2022 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"
	//	"path/filepath"
	//	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	// corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime"
	//"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	//clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	// ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	//	meta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

	fleetv1 "github.com/squaremo/fleeet/module/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
)

var _ = Describe("remote assemblage controller", func() {

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

		remoteasmReconciler := &RemoteAssemblageReconciler{
			Client: manager.GetClient(),
			Log:    ctrl.Log.WithName("controllers").WithName("BootstrapModule"),
			Scheme: manager.GetScheme(),
		}
		Expect(remoteasmReconciler.SetupWithManager(manager)).To(Succeed())

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
		namespace         corev1.Namespace
		clusterName       string
		clusterSecretName string
		source1           sourcev1.GitRepository
		source2           sourcev1.GitRepository
		asm               fleetv1.RemoteAssemblage
		syncs             []fleetv1.RemoteSync
	)

	BeforeEach(func() {
		// this doesn't get used, it just needs to be supplied
		clusterName = "cluster-" + randString(8)
		clusterSecretName = clusterName + "-kubeconfig" // NB this coincides with the hack used to elide between clusters and their secrets, which itself corresponds to what ClusterAPI does

		namespace = corev1.Namespace{}
		namespace.Name = randString(5)
		Expect(k8sClient.Create(context.TODO(), &namespace)).To(Succeed())

		// create a git repository for eah sync to refer to; this would usually be done by whatever
		// creates the assemblage (e.g., the bootstrap module controller)
		source1 = sourcev1.GitRepository{}
		source1.Name = "src-" + randString(5)
		source1.Namespace = namespace.Name
		source1.Spec.URL = "ssh://git@github.com/cuttlefacts/cuttlefacts-app"
		Expect(k8sClient.Create(context.TODO(), &source1)).To(Succeed())

		source2 = sourcev1.GitRepository{}
		source2.Name = "src-" + randString(5)
		source2.Namespace = namespace.Name
		source2.Spec.URL = "ssh://git@github.com/cuttlefacts/cuttlefacts-platform"
		Expect(k8sClient.Create(context.TODO(), &source2)).To(Succeed())

		syncs = []fleetv1.RemoteSync{
			{
				Name: "mostly-default",
				// TODO: control plane bindings
				SourceRef: fleetv1.SourceReference{
					Name:       source2.Name,
					APIVersion: "source.toolkit.fluxcd.io/v1beta1", // <-- could be constructed from imported vars
					Kind:       "GitRepository",
				},
				// .package left to default
			},
			{
				Name: "with-package-and-bindings",
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
				SourceRef: fleetv1.SourceReference{
					Name:       source2.Name,
					APIVersion: "source.toolkit.fluxcd.io/v1beta1", // <-- as above
					Kind:       "GitRepository",
				},
				Package: &syncapi.PackageSpec{
					Kustomize: &syncapi.KustomizeSpec{
						Path: "./app",
						Substitute: map[string]string{
							"foo": "${cluster.name}-foo",
						},
					},
				},
			},
		}

		// create an assemblage to serve as the input
		asm = fleetv1.RemoteAssemblage{
			Spec: fleetv1.RemoteAssemblageSpec{
				KubeconfigRef: fleetv1.LocalKubeconfigReference{Name: clusterSecretName},
				Syncs:         syncs,
			},
		}
		asm.Name = "asm-" + randString(5)
		asm.Namespace = namespace.Name
		Expect(k8sClient.Create(context.TODO(), &asm)).To(Succeed())
	})

	It("creates a kustomization per sync", func() {
		var kustoms kustomv1.KustomizationList
		Eventually(func() bool {
			if err := k8sClient.List(context.TODO(), &kustoms, client.InNamespace(namespace.Name)); err != nil {
				return false
			}
			return len(kustoms.Items) >= len(syncs)
		}, "5s", "1s").Should(BeTrue())
		Expect(len(kustoms.Items)).To(Equal(len(syncs)))

		// Check each Kustomization is controller-owned by the assemblage, targets the cluster, and
		// has the specified sync with expanded bindings.

		for _, kustom := range kustoms.Items {
			// the kustomization spec is what the module says
			Expect(kustom.Spec.SourceRef.Kind).To(Equal("GitRepository"))
			// the assemblage owns the kustomization
			controller := metav1.GetControllerOf(&kustom)
			Expect(controller).NotTo(BeNil())
			Expect(controller.Kind).To(Equal("RemoteAssemblage"))
			Expect(controller.Name).To(Equal(asm.Name))

			Expect(kustom.Spec.KubeConfig).ToNot(BeNil())
			Expect(kustom.Spec.KubeConfig.SecretRef.Name).To(Equal(clusterSecretName))

			// bindings are expanded, where they exist in the original sync
			if kustom.Spec.SourceRef.Name == source1.Name {
				Expect(kustom.Spec.PostBuild).NotTo(BeNil())
				Expect(kustom.Spec.PostBuild.Substitute).NotTo(BeNil())
				substitutions := kustom.Spec.PostBuild.Substitute
				Expect(substitutions).To(Equal(map[string]string{
					"cluster.name": clusterName,
				}))
			}
		}
	})
})

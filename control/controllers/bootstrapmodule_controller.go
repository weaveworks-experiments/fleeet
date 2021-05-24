/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

	fleetv1 "github.com/squaremo/fleeet/control/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
)

// BootstrapModuleReconciler reconciles a BootstrapModule object
type BootstrapModuleReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=bootstrapmodules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=bootstrapmodules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=bootstrapmodules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BootstrapModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("bootstrapmodule", req.NamespacedName)

	var mod fleetv1.BootstrapModule
	if err := r.Get(ctx, req.NamespacedName, &mod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.V(1).Info("found BootstrapModule")

	// The job of this controller is to make sure each eligible
	// cluster has a sync primitive targeting it. That means, in
	// GitOps Toolkit terms, there is a Kustomization object using the
	// cluster kubeconfig secret for each cluster, and a GitRepository
	// for them to all use as a source.

	// Create (or update) a source at which to point the
	// kustomizations.

	var source sourcev1.GitRepository
	source.Namespace = mod.Namespace
	source.Name = mod.Name
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &source, func() error {
		if err := syncapi.PopulateGitRepositorySpecFromSync(&source.Spec, &mod.Spec.Sync); err != nil {
			return err
		}
		// This is a hack to work around https://github.com/fluxcd/source-controller/issues/315
		source.Spec.Reference.Branch = "main"
		return controllerutil.SetControllerReference(&mod, &source, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("created/updated GitRepository", "name", source.Name, "operation", op)
	// TODO set a condition saying the source is created

	// For each eligible cluster, create a kustomization
	var clusters clusterv1.ClusterList
	selector, err := metav1.LabelSelectorAsSelector(mod.Spec.Selector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("could not make selector from %v: %w", mod.Spec.Selector, err)
	}
	if err := r.List(ctx, &clusters, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     mod.Namespace,
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list selected clusters: %w", err)
	}

	kustomSpec, err := syncapi.KustomizationSpecFromPackage(mod.Spec.Sync.Package, source.GetName())
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, cluster := range clusters.Items {
		var kustom kustomv1.Kustomization
		kustom.Namespace = mod.GetNamespace()
		kustom.Name = fmt.Sprintf("%s-%s", mod.GetName(), cluster.GetName())
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &kustom, func() error {
			kustom.Spec = kustomSpec
			// each kustomization is controlled by the bootstrap
			// module; if the module goes, so does the kustomization
			if err := controllerutil.SetControllerReference(&mod, &kustom, r.Scheme); err != nil {
				return err
			}
			// each kustomization is also owned by the cluster it
			// targets, for the sake of good bookkeeping (and
			// indexing)
			if err := controllerutil.SetOwnerReference(&cluster, &kustom, r.Scheme); err != nil {
				return err
			}

			kustom.Spec.KubeConfig = &kustomv1.KubeConfig{}
			kustom.Spec.KubeConfig.SecretRef.Name = fmt.Sprintf("%s-kubeconfig", cluster.GetName())

			return nil
		})

		if err != nil {
			return ctrl.Result{}, err
		}

		log.V(1).Info("created/updated kustomization", "name", kustom.GetName(), "operation", op)
	}
	// TODO find any rogue kustomizations and delete them

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BootstrapModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1.BootstrapModule{}).

		// Enqueue all the BootstrapModule objects that pertain to a
		// particular cluster
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(r.modulesForCluster)).
		Complete(r)
}

// TODO smoodge this together with ModuleReconciler.modulesForCluster
func (r *BootstrapModuleReconciler) modulesForCluster(cluster client.Object) []reconcile.Request {
	ctx := context.Background()
	var modules fleetv1.BootstrapModuleList
	if err := r.List(ctx, &modules, client.InNamespace(cluster.GetNamespace())); err != nil {
		r.Log.Error(err, "getting list of modules")
		return nil
	}

	var requests []reconcile.Request
	for _, mod := range modules.Items {
		if mod.Spec.Selector == nil {
			continue
		}
		name := types.NamespacedName{
			Name:      mod.Name,
			Namespace: mod.Namespace,
		}
		selector, err := metav1.LabelSelectorAsSelector(mod.Spec.Selector)
		if err != nil {
			r.Log.Error(err, "making selector for module", "module", name)
			continue
		}
		if selector.Matches(labels.Set(cluster.GetLabels())) {
			requests = append(requests, reconcile.Request{
				NamespacedName: name,
			})
		}
	}
	return requests
}

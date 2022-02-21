/*
Copyright 2021, 2022 Michael Bridgen <mikeb@squaremobius.net>.
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

	//kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

	fleetv1 "github.com/squaremo/fleeet/module/api/v1alpha1"
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

//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch

// Reconcile moves the cluster closer to the desired state, for a particular
// BootstrapModule. Usually this means making sure each selected cluster has a remote assemblage
// containing the sync given by the module, referring to a source (GitRepository).
func (r *BootstrapModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("bootstrapmodule", req.NamespacedName)

	var mod fleetv1.BootstrapModule
	if err := r.Get(ctx, req.NamespacedName, &mod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.V(1).Info("found BootstrapModule")

	// Create (or update) a source at which to point the syncs.

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

	// For each eligible cluster, ensure there's a RemoteAssemblage

	// Find all the selected clusters
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

	sourceRef := fleetv1.SourceReference{
		Name:       source.Name,
		APIVersion: source.APIVersion,
		Kind:       source.Kind,
	}

	for _, cluster := range clusters.Items {
		asm := &fleetv1.RemoteAssemblage{}
		asm.Namespace = cluster.GetNamespace()
		asm.Name = cluster.GetName()

		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, asm, func() error {
			// Each RemoteAssemblage is owned by each of the modules
			// assigned to it. This is for the sake of indexing.
			if err := controllerutil.SetOwnerReference(&mod, asm, r.Scheme); err != nil {
				return err
			}
			// Each RemoteAssemblage is _specially_ owned by the
			// cluster to which it pertains. This is so that removing
			// the cluster will garbage collect the remote assemblage.
			if err := controllerutil.SetControllerReference(&cluster, asm, r.Scheme); err != nil {
				return err
			}
			asm.Spec.KubeconfigRef = fleetv1.LocalKubeconfigReference{
				Name: cluster.GetName() + "-kubeconfig", // FIXME refer to cluster instead?
			}
			syncs := asm.Spec.Syncs
			for i, sync := range syncs {
				if sync.Name == mod.Name {
					// NB: CreateOrUpdate will avoid the update if the mutated object
					// is deep-equal to the original. That helps this process reach a
					// fixed point.
					syncs[i].Package = mod.Spec.Sync.Package
					syncs[i].ControlPlaneBindings = mod.Spec.ControlPlaneBindings
					syncs[i].SourceRef = sourceRef
					return nil
				}
			}
			// not there -- add this module
			asm.Spec.Syncs = append(syncs, fleetv1.RemoteSync{
				Name:                 mod.Name,
				Package:              mod.Spec.Sync.Package,
				ControlPlaneBindings: mod.Spec.ControlPlaneBindings,
				SourceRef:            sourceRef,
			})
			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		log.V(1).Info("created/updated RemoteAssemblage", "name", asm.Name, "operation", op)
	}
	// TODO find any redundant sources and assemblages and delete them

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

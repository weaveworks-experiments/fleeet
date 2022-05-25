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

// These are what the controller creates:
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=remoteassemblages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch;create;update;patch;delete

// The controller watches these, to see when the selection changes
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
		// the resulting source is "controller owned" by the bootstrap module
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

	summary := &fleetv1.SyncSummary{}

	// Keep track of the assemblages which did require this module;
	// afterwards, this will be helpful to determine the assemblages
	// which need the module removed.
	requiredAsm := map[string]struct{}{}

clusters:
	for _, cluster := range clusters.Items {
		// Don't bother if the cluster isn't marked as ready yet. Creating an assemblage targeting
		// an unready cluster means lots of failures and back-off.
		if !cluster.Status.ControlPlaneReady {
			// TODO: should this be a separate field in the summary, e.g., "waiting"?
			log.Info("waiting for cluster to be ready", "name", cluster.GetName())
			continue
		}
		summary.Total++
		requiredAsm[cluster.GetName()] = struct{}{}

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
			summary.Failed++
			log.Error(err, "updating remote assemblages", "assemblage", asm.Name)
		} else {
			log.V(1).Info("created/updated RemoteAssemblage", "name", asm.Name, "operation", op)
			for _, sync := range asm.Status.Syncs {
				if sync.Name == mod.Name {
					switch sync.State {
					case syncapi.StateSucceeded:
						summary.Succeeded++
					case syncapi.StateFailed:
						summary.Failed++
					default:
						summary.Updating++
					}
					continue clusters // all done here
				}
			}
			// no change made, but status not found -> updating
			summary.Updating++
		}
	}

	// Find all assemblages indexed as owned by (i.e., including) this
	// module
	var asms fleetv1.RemoteAssemblageList
	if err := r.List(ctx, &asms, client.InNamespace(req.Namespace), client.MatchingFields{remoteAssemblageOwnerKey: req.Name}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing assemblages for this module: %w", err)
	}

	// This loop removes the bootstrap module from any assemblage for a cluster that _wasn't_
	// selected. (Remember, these assemblages were selected because they were owned by this module,
	// implying that at some point the module was assigned to the cluster)
	for _, asm := range asms.Items {
		if _, ok := requiredAsm[asm.GetName()]; !ok {
			syncs := asm.Spec.Syncs
			for i, sync := range syncs {
				if sync.Name == mod.Name {
					if _, err := controllerutil.CreateOrPatch(ctx, r.Client, &asm, func() error {
						asm.Spec.Syncs = append(syncs[:i], syncs[i+1:]...)
						removeOwnerRef(&mod, &asm)
						return nil
					}); err != nil {
						log.Error(err, "removing module from remote assemblage", "assemblage", asm.Name)
					}
					// FIXME: can this `break` from the loop at this point?
				}
			}
		}
	}

	mod.Status.ObservedSync = &mod.Spec.Sync
	mod.Status.Summary = summary
	if err := r.Status().Update(ctx, &mod); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status of bootstrap module: %w", err)
	}

	return ctrl.Result{}, nil
}

const remoteAssemblageOwnerKey = "ownerBootstrapModule"

// SetupWithManager sets up the controller with the Manager.
func (r *BootstrapModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// This index is to conveniently find the remote assemblages that a bootstrap module owns (i.e.,
	// is included in). The watch on RemoteAssemblages below will enqueue each bootstrap module
	// owner of a remote aseemblage that has been updated.
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &fleetv1.RemoteAssemblage{}, remoteAssemblageOwnerKey, func(obj client.Object) []string {
		asm := obj.(*fleetv1.RemoteAssemblage)
		var moduleOwners []string
		for _, owner := range asm.GetOwnerReferences() {
			// FIXME: make this more reliable? What are the consequences of getting another API's
			// BootstrapModule mixed in here? Something like this might be better:
			// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.11.1/pkg/handler/enqueue_owner.go#L46
			if owner.Kind == fleetv1.KindBootstrapModule {
				moduleOwners = append(moduleOwners, owner.Name)
			}
		}
		return moduleOwners
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1.BootstrapModule{}).
		Owns(&sourcev1.GitRepository{}).
		// These are not "controller-owned" by the bootstrap modules, so this cannot use `.Owns`
		Watches(
			&source.Kind{Type: &fleetv1.RemoteAssemblage{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &fleetv1.BootstrapModule{},
				IsController: false,
			}).

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

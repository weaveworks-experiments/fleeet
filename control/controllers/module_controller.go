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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
	fleetv1 "github.com/squaremo/fleeet/control/api/v1alpha1"
)

// ModuleReconciler reconciles a Module object
type ModuleReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	assemblageOwnerKey = "ownerModule"
)

//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=modules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=modules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=modules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("module", req.NamespacedName)

	var mod fleetv1.Module
	if err := r.Get(ctx, req.NamespacedName, &mod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// --- update this module's status ---

	// Find all assemblages that include this module
	var asms fleetv1.RemoteAssemblageList
	if err := r.List(ctx, &asms, client.InNamespace(req.Namespace), client.MatchingFields{assemblageOwnerKey: req.Name}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing assemblages for this module: %w", err)
	}

	summary := &fleetv1.SyncSummary{}
	for _, asm := range asms.Items {
		for _, s := range asm.Status.Syncs {
			if s.Sync.Name == mod.Name {
				switch s.State {
				case asmv1.StateSucceeded:
					summary.Succeeded++
				case asmv1.StateFailed:
					summary.Failed++
				case asmv1.StateUpdating:
					summary.Updating++
				}
			}
		}
		summary.Total++
	}
	mod.Status.Summary = summary
	if err := r.Status().Update(ctx, &mod); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status of module: %w", err)
	}

	// --- create/update/delete remote assemblages

	// Make sure there is a remote assemblage which includes this
	// module, for every cluster that matches the selector.

	var clusters clusterv1.ClusterList
	// `LabelSelectorAsSelector` correctly handles nil and empty
	// selector values by selecting nothing and everything,
	// respectively.
	selector, err := metav1.LabelSelectorAsSelector(mod.Spec.Selector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("could not make selector from %v: %w", mod.Spec.Selector, err)
	}
	if err := r.List(ctx, &clusters, &client.ListOptions{
		LabelSelector: selector,
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list selected clusters: %w", err)
	}

	// Keep track of the assemblages which did require this module;
	// afterwards, this will be helpful to determine the assemblages
	// which need the module removed.
	requiredAsm := map[string]struct{}{}
	for _, cluster := range clusters.Items {
		// This loop makes sure every cluster that matches the
		// selector has a remote assemblage with the latest definition
		// of the module, by either updating an existing assemblage or
		// creating one.
		requiredAsm[cluster.GetName()] = struct{}{}

		asm := &fleetv1.RemoteAssemblage{}
		asm.Namespace = cluster.GetNamespace()
		asm.Name = cluster.GetName()

		if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, asm, func() error {
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
				Name: cluster.GetName() + "-kubeconfig", // FIXME refer to cluster instead
			}

			// if this module is to be found in the syncs, make sure
			// it's the up to date definition.
			syncs := asm.Spec.Assemblage.Syncs
			for i, sync := range syncs {
				if sync.Name == mod.Name {
					// NB: CreateOrUpdate will avoid the update if the mutated object
					// is deep-equal to the original. That helps this process reach a
					// fixed point.
					syncs[i].Sync = mod.Spec.Sync
					return nil
				}
			}
			// not there -- add this module
			asm.Spec.Assemblage.Syncs = append(syncs, asmv1.NamedSync{
				Name: mod.Name,
				Sync: mod.Spec.Sync,
			})
			return nil
		}); err != nil {
			log.Error(err, "updating remote assemblages", "assemblage", asm.Name)
		} else {
			log.V(1).Info("updated assemblage", "assemblage", asm.Name, "operation", op)
		}
	}

	// This loop removes the module from any assemblage for a cluster
	// that wasn't selected. (Remember, these assemblages were
	// selected because they were owned by this module, implying that
	// at some point the module was assigned to the cluster)
	for _, asm := range asms.Items {
		if _, ok := requiredAsm[asm.GetName()]; !ok {
			syncs := asm.Spec.Assemblage.Syncs
			for i, sync := range syncs {
				if sync.Name == mod.Name {
					asm.Spec.Assemblage.Syncs = append(syncs[:i], syncs[i+1:]...)
					removeOwnerRef(&mod, &asm)
					if err := r.Update(ctx, &asm); err != nil {
						log.Error(err, "removing module from remote assemblage", "assemblage", asm.Name)
					}
					// FIXME: can this `break` from the loop at this point?
				}
			}
		}
	}

	// TODO: This should correspond to the summary; figure out if the
	// summary should be calculated based on changes done above.
	mod.Status.ObservedSync = &mod.Spec.Sync
	if err := r.Status().Update(ctx, &mod); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status of module: %w", err)
	}

	return ctrl.Result{}, nil
}

func removeOwnerRef(nonOwner, obj metav1.Object) {
	owners := obj.GetOwnerReferences()
	newOwners := make([]metav1.OwnerReference, len(owners))
	removeUID := nonOwner.GetUID()
	for i := range owners {
		if owners[i].UID != removeUID {
			newOwners = append(newOwners, owners[i])
		}
	}
	obj.SetOwnerReferences(newOwners)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// This sets up an index on the Module owners of RemoteAssemblage
	// objects. This complements the Watch on assemblage owners,
	// below: that enqueues all the modules related to an assemblage
	// that has changed, while this helps get the assemblages related
	// to a module.
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &fleetv1.RemoteAssemblage{}, assemblageOwnerKey, func(obj client.Object) []string {
		asm := obj.(*fleetv1.RemoteAssemblage)
		var moduleOwners []string
		for _, owner := range asm.GetOwnerReferences() {
			// FIXME: make this more reliable? What are the
			// consequences of getting another API's Module mixed in
			// here?
			if owner.Kind == fleetv1.KindModule {
				moduleOwners = append(moduleOwners, owner.Name)
			}
		}
		return moduleOwners
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1.Module{}).

		// Enqueue a Module any time a RemoteAssemblage that records
		// it as an owner is changed. This cannot use "Owns" because
		// more than one module can be an owner of a RemoteAssemblage
		// (and none will be the controller owner).
		Watches(
			&source.Kind{Type: &fleetv1.RemoteAssemblage{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &fleetv1.Module{},
				IsController: false,
			}).

		// Enqueue all the Module objects that pertain to a
		// particular cluster
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(r.modulesForCluster)).
		Complete(r)
}

func (r *ModuleReconciler) modulesForCluster(cluster client.Object) []reconcile.Request {
	ctx := context.Background()
	var modules fleetv1.ModuleList
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

/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
	fleetv1 "github.com/squaremo/fleeet/module/api/v1alpha1"
)

// RemoteAssemblageReconciler reconciles a RemoteAssemblage object
type RemoteAssemblageReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// cache is a remote cluster client cache
	cache *remote.ClusterCacheTracker
}

//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=remoteassemblages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=remoteassemblages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=remoteassemblages/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;

// FIXME: access to secrets?

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RemoteAssemblageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("remoteassemblage", req.NamespacedName)

	var asm fleetv1.RemoteAssemblage
	if err := r.Get(ctx, req.NamespacedName, &asm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Let's go looking for the corresponding assemblage in the remote
	// cluster.
	clusterKey := client.ObjectKey{
		Namespace: asm.Namespace,
		// HACK: the client cache accepts cluster keys, but we are
		// ging straight to the secret; the trim gets the former from
		// the latter.
		Name: strings.TrimSuffix(asm.Spec.KubeconfigRef.Name, "-kubeconfig"),
	}
	remoteClient, err := r.cache.GetClient(ctx, clusterKey)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("could not get client for remote cluster: %w", err)
	}

	log.V(1).Info("remote cluster connected", "cluster", clusterKey.Name)

	var counterpart asmv1.Assemblage
	counterpart.Name = asm.Name
	counterpart.Namespace = asm.Namespace
	op, err := controllerutil.CreateOrUpdate(ctx, remoteClient, &counterpart, func() error {
		counterpart.Spec = asm.Spec.Assemblage
		return nil
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("while create/update counterpart in downstream: %w", err)
	}

	switch op {
	case controllerutil.OperationResultNone,
		controllerutil.OperationResultUpdated:
		asm.Status.Syncs = counterpart.Status.Syncs
	case controllerutil.OperationResultCreated:
		// TODO set a condition saying the downstream is created
	}

	if err = r.Status().Update(ctx, &asm); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteAssemblageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := remote.NewClusterCacheTracker(mgr.GetLogger(), mgr)
	if err != nil {
		return err
	}
	r.cache = c

	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1.RemoteAssemblage{}).
		Complete(r)
}

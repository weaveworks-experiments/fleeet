/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	//kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

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

	var source sourcev1.GitRepository
	source.Namespace = mod.Namespace
	source.Name = mod.Name
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &source, func() error {
		return syncapi.PopulateGitRepositorySpecFromSync(&source.Spec, &mod.Spec.Sync)
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("created/updated GitRepository", "name", source.Name, "operation", op)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BootstrapModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1.BootstrapModule{}).
		Complete(r)
}

/*
Copyright 2021 Michael Bridgen
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
)

// AssemblageReconciler reconciles a Assemblage object
type AssemblageReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=assemblages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=assemblages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=assemblages/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Assemblage object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *AssemblageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("assemblage", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AssemblageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1alpha1.Assemblage{}).
		Complete(r)
}

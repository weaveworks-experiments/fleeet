/*
Copyright 2021 Michael Bridgen
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	fleetv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
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
func (r *AssemblageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("assemblage", req.NamespacedName)

	// Get the Assemblage in question
	var asm fleetv1.Assemblage
	if err := r.Get(ctx, req.NamespacedName, &asm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// For each sync, make sure the correct GitOps Toolkit objects
	// exist
	for i, sync := range asm.Spec.Syncs {
		// Firstly, a source
		var source sourcev1.GitRepository
		source.Namespace = asm.Namespace
		source.Name = fmt.Sprintf("%s-%d", asm.Name, i)

		if _, err := ctrl.CreateOrUpdate(ctx, r.Client, &source, func() error {
			spec, err := gitRepositorySpecFromSync(&sync)
			if err != nil {
				return err
			}
			source.Spec = spec
			// TODO set the owner
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("creating source git repository", "name", source.Name)

		// TODO Secondly, a Kustomization
	}

	// TODO For each GitOps Toolkit sync object, collect the status

	return ctrl.Result{}, nil
}

func gitRepositorySpecFromSync(sync *fleetv1.Sync) (sourcev1.GitRepositorySpec, error) {
	var dstSpec sourcev1.GitRepositorySpec
	srcSpec := sync.Source.Git
	dstSpec.URL = srcSpec.URL
	dstSpec.Interval = metav1.Duration{Duration: time.Minute} // TODO arbitrary

	var ref sourcev1.GitRepositoryRef
	if tag := srcSpec.Version.Tag; tag != "" {
		ref.Tag = tag
	} else if rev := srcSpec.Version.Revision; rev != "" {
		ref.Commit = rev
	} else {
		return dstSpec, fmt.Errorf("neither tag nor revision given in git source spec")
	}
	dstSpec.Reference = &ref

	return dstSpec, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AssemblageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1.Assemblage{}).
		Complete(r)
}

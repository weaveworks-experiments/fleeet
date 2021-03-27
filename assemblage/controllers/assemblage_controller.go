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

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
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
			spec, err := gitRepositorySpecFromSync(&sync.Sync)
			if err != nil {
				return err
			}
			source.Spec = spec
			// TODO set the owner
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("creating/updating source git repository", "name", source.Name)

		// Secondly, a Kustomization
		switch {
		case sync.Package.Kustomize != nil:
			var kustom kustomv1.Kustomization
			kustom.Namespace = asm.Namespace
			kustom.Name = fmt.Sprintf("%s-%d", asm.Name, i)

			if _, err := ctrl.CreateOrUpdate(ctx, r.Client, &kustom, func() error {
				spec, err := kustomizationSpecFromPackage(sync.Package, source.Name)
				if err != nil {
					return err
				}
				kustom.Spec = spec
				return nil
			}); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("creating/updating kustomization", "name", kustom.Name)
		default:
			log.Info("no sync package present", "sync", i)
		}
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
		ref.Branch = "main" // FIXME a hack to make it work with my repos, see https://github.com/fluxcd/source-controller/issues/315
		ref.Commit = rev
	} else {
		return dstSpec, fmt.Errorf("neither tag nor revision given in git source spec")
	}
	dstSpec.Reference = &ref

	return dstSpec, nil
}

func kustomizationSpecFromPackage(pkg *fleetv1.PackageSpec, sourceName string) (kustomv1.KustomizationSpec, error) {
	var spec kustomv1.KustomizationSpec
	spec.SourceRef = kustomv1.CrossNamespaceSourceReference{
		Kind: sourcev1.GitRepositoryKind,
		Name: sourceName,
	}
	spec.Path = pkg.Kustomize.Path
	return spec, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AssemblageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1.Assemblage{}).
		Complete(r)
}

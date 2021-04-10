/*
Copyright 2021 Michael Bridgen
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/go-logr/logr"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
	// exist, and collect the status of any that do.
	var statuses []fleetv1.SyncStatus
	for i, sync := range asm.Spec.Syncs {
		syncStatus := fleetv1.SyncStatus{
			Sync: sync,
		}

		// Firstly, a source
		var source sourcev1.GitRepository
		source.Namespace = asm.Namespace
		source.Name = fmt.Sprintf("%s-%d", asm.Name, i) // TODO is the order stable?

		op, err := ctrl.CreateOrUpdate(ctx, r.Client, &source, func() error {
			if err := populateGitRepositorySpecFromSync(&source.Spec, &sync.Sync); err != nil {
				return err
			}
			if err := controllerutil.SetControllerReference(&asm, &source, r.Scheme); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		log.Info("creating/updating source git repository", "name", source.Name, "operation", op)

		// If the source changed, it's all updating
		switch op {
		case controllerutil.OperationResultCreated,
			controllerutil.OperationResultUpdated,
			controllerutil.OperationResultUpdatedStatus:
			syncStatus.State = fleetv1.StateUpdating
		case controllerutil.OperationResultNone:
			break
		default:
			log.V(1).Info("unhandled operation result", "operation", op)
		}

		// Secondly, a Kustomization
		switch {
		case sync.Package.Kustomize != nil:
			var kustom kustomv1.Kustomization
			kustom.Namespace = asm.Namespace
			kustom.Name = fmt.Sprintf("%s-%d", asm.Name, i)

			op, err := ctrl.CreateOrUpdate(ctx, r.Client, &kustom, func() error {
				spec, err := kustomizationSpecFromPackage(sync.Package, source.Name)
				if err != nil {
					return err
				}
				kustom.Spec = spec
				if err = controllerutil.SetControllerReference(&asm, &kustom, r.Scheme); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Info("creating/updating kustomization", "name", kustom.Name, "operation", op)
			// the source might be unready above, in which case the
			// aggregate state is updating; but if not, it'll be down
			// to the kustomization's ready state
			if syncStatus.State == "" {
				switch op {
				case controllerutil.OperationResultNone:
					syncStatus.State = readyState(&kustom)
				default:
					syncStatus.State = fleetv1.StateUpdating
				}
			}
		default:
			log.Info("no sync package present", "sync", i)
		}
		statuses = append(statuses, syncStatus)
	}

	asm.Status.Syncs = statuses
	if err := r.Status().Update(ctx, &asm); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func readyState(obj meta.ObjectWithStatusConditions) fleetv1.SyncState {
	conditions := obj.GetStatusConditions()
	c := apimeta.FindStatusCondition(*conditions, meta.ReadyCondition)
	switch {
	case c == nil:
		return fleetv1.StateUpdating
	case c.Status == metav1.ConditionTrue:
		return fleetv1.StateSucceeded
	case c.Status == metav1.ConditionFalse:
		if c.Reason == meta.ReconciliationFailedReason {
			return fleetv1.StateFailed
		} else {
			return fleetv1.StateUpdating
		}
	default: // FIXME possibly StateUnknown?
		return fleetv1.StateUpdating
	}
}

func populateGitRepositorySpecFromSync(dst *sourcev1.GitRepositorySpec, sync *fleetv1.Sync) error {
	srcSpec := sync.Source.Git
	dst.URL = srcSpec.URL
	dst.Interval = metav1.Duration{Duration: time.Minute} // TODO arbitrary

	ref := *dst.Reference
	if tag := srcSpec.Version.Tag; tag != "" {
		ref.Tag = tag
	} else if rev := srcSpec.Version.Revision; rev != "" {
		ref.Commit = rev
	} else {
		return fmt.Errorf("neither tag nor revision given in git source spec")
	}
	dst.Reference = &ref

	return nil
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
		Owns(&sourcev1.GitRepository{}).
		Owns(&kustomv1.Kustomization{}).
		Complete(r)
}

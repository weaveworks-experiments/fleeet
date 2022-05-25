/*
Copyright 2021 Michael Bridgen
*/

package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

	asmv1 "github.com/squaremo/fleeet/assemblage/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
)

// ErrCircularBinding is the error that will be logged if binding
// definitions refer to each other recursively.
var ErrCircularBinding = errors.New("circular definition of binding")

// AssemblageReconciler reconciles a Assemblage object
type AssemblageReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=assemblages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=assemblages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=fleet.squaremo.dev,resources=assemblages/finalizers,verbs=update
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=kustomize.toolkit.fluxcd.io,resources=kustomizations,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AssemblageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("assemblage", req.NamespacedName)

	// Get the Assemblage in question
	var asm asmv1.Assemblage
	if err := r.Get(ctx, req.NamespacedName, &asm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	namespacedClient := client.NewNamespacedClient(r.Client, asm.Namespace)

	// For each sync, make sure the correct GitOps Toolkit objects
	// exist, and collect the status of any that do.
	var statuses []syncapi.SyncStatus
	for i, sync := range asm.Spec.Syncs {
		syncStatus := syncapi.SyncStatus{
			Sync: sync,
		}

		// Firstly, a source
		var source sourcev1.GitRepository
		source.Namespace = asm.Namespace
		source.Name = fmt.Sprintf("%s-%d", asm.Name, i) // TODO is the order stable?

		op, err := ctrl.CreateOrUpdate(ctx, r.Client, &source, func() error {
			if err := syncapi.PopulateGitRepositorySpecFromSync(&source.Spec, &sync.Sync); err != nil {
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
			syncStatus.State = syncapi.StateUpdating
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
				spec, err := syncapi.KustomizationSpecFromPackage(sync.Package, source.Name, makeBindingFunc(ctx, log, namespacedClient, sync.Bindings, nil))
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
					syncStatus.State = syncapi.KustomizeReadyState(&kustom)
				default:
					syncStatus.State = syncapi.StateUpdating
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

func makeBindingFunc(ctx context.Context, log logr.Logger, namespacedClient client.Client, bindings []syncapi.Binding, stack []string) func(string) string {
	memo := map[string]string{}
	return func(name string) string {
		// if this name was mentioned before, there's a circular definition
		for i := range stack {
			if stack[i] == name {
				log.Error(ErrCircularBinding, "name", name)
				// FIXME no way to bail; do the usual thing and pretend it doesn't exist
				memo[name] = ""
				return ""
			}
		}

		if v, ok := memo[name]; ok {
			return v
		}

		for _, b := range bindings {
			if b.Name == name {
				v, err := syncapi.ResolveBinding(ctx, namespacedClient, b, makeBindingFunc(ctx, log, namespacedClient, bindings, append(stack, name)))
				if err != nil {
					log.Info("warning: unable to resolve binding; using empty string", "name", b.Name, "error", err)
					v = ""
				}
				memo[name] = v
				return v
			}
		}
		memo[name] = ""
		return ""
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AssemblageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&asmv1.Assemblage{}).
		Owns(&sourcev1.GitRepository{}).
		Owns(&kustomv1.Kustomization{}).
		Complete(r)
}

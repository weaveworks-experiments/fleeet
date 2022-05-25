/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"

	fleetv1 "github.com/squaremo/fleeet/module/api/v1alpha1"
	syncapi "github.com/squaremo/fleeet/pkg/api"
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

// The controller creates these:
//+kubebuilder:rbac:groups=kustomize.toolkit.fluxcd.io,resources=kustomizations,verbs=get;list;watch;create;update;patch;delete

// The controller watches these, to see when it might need to retry for a cluster that was missing its secret
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile moves the cluster closer to the desired state, as specified in the named
// RemoteAssemblage. Usually this means making sure each sync in the assemblage has an up-to-date
// Flux primitive to represent it.
func (r *RemoteAssemblageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("remoteassemblage", req.NamespacedName)

	var asm fleetv1.RemoteAssemblage
	if err := r.Get(ctx, req.NamespacedName, &asm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	clusterName := asm.Spec.KubeconfigRef.Name
	clusterName = clusterName[:len(clusterName)-len("-kubeconfig")] // FIXME this is a hack, while types refer to kubeconfig secrets rather than e.g., clusters

	// Check if the kubeconfig secret exists. This is a workaround of a sort, because this
	// controller creates Flux primitives referring to the secret, and the Flux controllers should
	// back off when the secret is not available. However, a new Cluster object typically appears
	// minutes before its secret, by which time back-off intervals can become significant. To be
	// more responsive, the following will balk without creating Flux primitives when the secret
	// isn't present, and rely on the secret creation event to trigger another reconciliation.

	// TODO: this early exit won't clean up Flux primitives that were created before a secret went
	// missing.
	var kubeconfig corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: req.NamespacedName.Namespace,
		Name:      asm.Spec.KubeconfigRef.Name,
	}, &kubeconfig); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		// TODO: mark as stalled using condition
		log.Info("secret not found", "name", asm.Spec.KubeconfigRef.Name)
		return ctrl.Result{}, nil
	}

	var syncStatus []fleetv1.SyncStatus

	// Used to get any resources mentioned in controlPlaneBindings
	namespacedClient := client.NewNamespacedClient(r.Client, asm.GetNamespace())
	for _, sync := range asm.Spec.Syncs {
		// start with CLUSTER_NAME available to use in bindings
		memo := map[string]string{
			"CLUSTER_NAME": clusterName,
		}

		var bindingErr error
		var makeBindingFunc func(stack []string) func(string) string
		makeBindingFunc = func(stack []string) func(string) string {
			return func(name string) string {
				for i := range stack {
					if stack[i] == name {
						bindingErr = fmt.Errorf("circular binding %q", name)
						return ""
					}
				}

				if v, ok := memo[name]; ok {
					return v
				}
				for _, b := range sync.ControlPlaneBindings {
					if b.Name == name {
						v, err := syncapi.ResolveBinding(ctx, namespacedClient, b, makeBindingFunc(append(stack, name)))
						if err != nil {
							bindingErr = err
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

		kustomSpec, err := syncapi.KustomizationSpecFromPackage(sync.Package, sync.SourceRef.Name, makeBindingFunc(nil))
		if err != nil {
			return ctrl.Result{}, err
		}
		if bindingErr != nil {
			return ctrl.Result{}, bindingErr
		}

		var kustom kustomv1.Kustomization
		kustom.Namespace = asm.GetNamespace()
		kustom.Name = fmt.Sprintf("%s-%s", sync.Name, clusterName) // FIXME may need to hash one or both to limit size
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &kustom, func() error {
			kustom.Spec = kustomSpec
			// each kustomization is controlled by the assemblage; if the asssemblage goes, so does the kustomization
			if err := controllerutil.SetControllerReference(&asm, &kustom, r.Scheme); err != nil {
				return err
			}

			kustom.Spec.KubeConfig = &kustomv1.KubeConfig{}
			kustom.Spec.KubeConfig.SecretRef.Name = asm.Spec.KubeconfigRef.Name

			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		log.V(1).Info("created/updated kustomization", "name", kustom.GetName(), "operation", op)
		status := fleetv1.SyncStatus{
			Name: sync.Name,
		}
		switch op {
		case controllerutil.OperationResultNone: // nothing was changed, so the sync state should reflect the status of the Flux primitive
			status.State = syncapi.KustomizeReadyState(&kustom)
		default:
			status.State = syncapi.StateUpdating
		}
		syncStatus = append(syncStatus, status)
	}

	asm.Status.Syncs = syncStatus
	if err := r.Status().Update(ctx, &asm); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

const kubeconfigSecretField = ".spec.kubeconfigRef.name"

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteAssemblageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := remote.NewClusterCacheTracker(mgr.GetLogger(), mgr)
	if err != nil {
		return err
	}
	r.cache = c

	// This index keeps track of RemoteAssemblage objects that reference a given secret. When a
	// secret is created or updated, those assemblage objects need to be examined.
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &fleetv1.RemoteAssemblage{}, kubeconfigSecretField, func(raw client.Object) []string {
		asm := raw.(*fleetv1.RemoteAssemblage)
		return []string{asm.Spec.KubeconfigRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1.RemoteAssemblage{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.assemblagesForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Owns(&kustomv1.Kustomization{}).
		Complete(r)
}

func (r *RemoteAssemblageReconciler) assemblagesForSecret(secret client.Object) []reconcile.Request {
	var list fleetv1.RemoteAssemblageList
	options := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(kubeconfigSecretField, secret.GetName()),
		Namespace:     secret.GetNamespace(),
	}
	if err := r.List(context.TODO() /* ugh */, &list, options); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, len(list.Items))
	for i, item := range list.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: item.GetNamespace(),
				Name:      item.GetName(),
			},
		}
	}
	return requests
}

/*
Copyright 2021 Michael Bridgen <mikeb@squaremobius.net>.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kustomize.toolkit.fluxcd.io,resources=kustomizations,verbs=get;list;watch;create;update;patch;delete

// Reconcile moves the cluster closer to the desired state, as specified in the named
// RemoteAssemblage. Usually this means making sure each sync in the assemblage has an up-to-date
// Flux primitive to represent it.
func (r *RemoteAssemblageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("remoteassemblage", req.NamespacedName)

	var asm fleetv1.RemoteAssemblage
	if err := r.Get(ctx, req.NamespacedName, &asm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO: go through the syncs and create a Kustomization object for each sync, pointing at the
	// source it references.
	clusterName := asm.Spec.KubeconfigRef.Name
	clusterName = clusterName[:len(clusterName)-len("-kubeconfig")] // FIXME this is a hack, while types refer to kubeconfig secrets rather than e.g., clusters

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
	}

	// TODO: report that status of each sync

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

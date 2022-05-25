package api

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

	"github.com/squaremo/fleeet/pkg/expansion"
)

func KustomizationSpecFromPackage(pkg *PackageSpec, sourceName string, mapping func(string) string) (kustomv1.KustomizationSpec, error) {
	var spec kustomv1.KustomizationSpec
	spec.SourceRef = kustomv1.CrossNamespaceSourceReference{
		Kind: sourcev1.GitRepositoryKind,
		Name: sourceName,
	}
	spec.Path = pkg.Kustomize.Path

	if subSpec := pkg.Kustomize.Substitute; subSpec != nil {
		substitutions := map[string]string{}
		for k, v := range subSpec {
			substitutions[k] = expansion.Expand(v, mapping)
		}
		spec.PostBuild = &kustomv1.PostBuild{
			Substitute: substitutions,
		}
	}
	return spec, nil
}

func KustomizeReadyState(obj *kustomv1.Kustomization) SyncState {
	conditions := obj.GetStatusConditions()
	c := apimeta.FindStatusCondition(*conditions, fluxmeta.ReadyCondition)
	switch {
	case c == nil:
		return StateUpdating
	case c.Status == metav1.ConditionTrue:
		return StateSucceeded
	case c.Status == metav1.ConditionFalse:
		if c.Reason == fluxmeta.ReconciliationFailedReason {
			return StateFailed
		} else {
			return StateUpdating
		}
	default: // FIXME possibly StateUnknown?
		return StateUpdating
	}
}

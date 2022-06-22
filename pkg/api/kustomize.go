package api

import (
	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
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
	//TODO: make this overideable
	spec.TargetNamespace = "default"

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

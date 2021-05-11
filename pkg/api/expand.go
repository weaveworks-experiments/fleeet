package api

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kustomv1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
)

func PopulateGitRepositorySpecFromSync(dst *sourcev1.GitRepositorySpec, sync *Sync) error {
	srcSpec := sync.Source.Git
	dst.URL = srcSpec.URL
	dst.Interval = metav1.Duration{Duration: time.Minute} // TODO arbitrary

	var ref sourcev1.GitRepositoryRef
	if dst.Reference != nil {
		ref = *dst.Reference
	}
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

func KustomizationSpecFromPackage(pkg *PackageSpec, sourceName string) (kustomv1.KustomizationSpec, error) {
	var spec kustomv1.KustomizationSpec
	spec.SourceRef = kustomv1.CrossNamespaceSourceReference{
		Kind: sourcev1.GitRepositoryKind,
		Name: sourceName,
	}
	spec.Path = pkg.Kustomize.Path
	return spec, nil
}

module github.com/squaremo/fleeet/assemblage

go 1.15

require (
	github.com/fluxcd/kustomize-controller/api v0.12.0
	github.com/fluxcd/source-controller/api v0.12.2
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/squaremo/fleeet/pkg v0.0.3-rc1
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.20.4
	sigs.k8s.io/controller-runtime v0.8.3
)

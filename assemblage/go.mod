module github.com/squaremo/fleeet/assemblage

go 1.15

// until there is a tagged version of this
replace github.com/squaremo/fleeet/pkg => ../pkg

require (
	github.com/fluxcd/kustomize-controller/api v0.9.3
	github.com/fluxcd/pkg/apis/meta v0.8.0
	github.com/fluxcd/source-controller/api v0.9.0
	github.com/go-logr/logr v0.3.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/squaremo/fleeet/pkg v0.0.0-00010101000000-000000000000
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)

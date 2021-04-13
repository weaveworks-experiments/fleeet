module github.com/squaremo/fleeet/control

go 1.15

// until there is a tagged version of this
replace github.com/squaremo/fleeet/pkg => ../pkg

replace github.com/squaremo/fleeet/assemblage => ../assemblage

require (
	github.com/fluxcd/kustomize-controller/api v0.9.3
	github.com/fluxcd/source-controller/api v0.9.0
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/squaremo/fleeet/assemblage v0.0.2
	github.com/squaremo/fleeet/pkg v0.0.0-00010101000000-000000000000
	k8s.io/api v0.21.0-beta.1
	k8s.io/apimachinery v0.21.0-beta.1
	k8s.io/client-go v0.21.0-beta.1
	sigs.k8s.io/cluster-api v0.3.11-0.20210323155336-f39a263d435c
	sigs.k8s.io/controller-runtime v0.9.0-alpha.0
)

module github.com/squaremo/fleeet/control

go 1.15

replace github.com/squaremo/fleeet/assemblage => ../assemblage

require (
	github.com/go-logr/logr v0.3.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/squaremo/fleeet/assemblage v0.0.0-00010101000000-000000000000
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)

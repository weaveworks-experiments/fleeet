<!-- -*- fill-column: 100 -*- -->
# Fleeet documentation

 1. What Fleeet is for
 2. How Fleeet works
 3. How to install Fleeet
 4. How to use Fleeet

## What is this for?

This is for applying configuration automatically to a dynamic set of Kubernetes clusters. By
"dynamic" I mean that clusters in the set can appear, change their properties, and disappear,
arbitrarily.

[Flux](https://github.com/fluxcd/flux2) and its peers will automatically apply configuration to an
explicitly given cluster. Dealing with _sets_ of clusters brings a few extra challenges:

 - you don't want to have to write out or remove definitions when clusters come and go;
 - often, a configuration must be adapted to each cluster, and if there are arbitrary numbers of
   clusters, it is dreary to do this by hand;
 - if you applied each new configuration to all the clusters at once, the blast radius of a bad
   change would be all the clusters -- so, you need to roll updates out to clusters gradually,
   similar to how Kubernetes' Deployments work;
 - co-ordination is needed to do rollouts, and that has to happen somewhere.

## How does it work?

Fleeet runs as a [controller] in a control-plane cluster. It keeps track of workload clusters by
watching for Cluster API `Cluster` objects. You create and update `Module` objects, which refer to a
piece of configuration at a specific version. Fleeet applies the configuration for each `Module` to
each workload cluster.

Sometimes you only want a configuration to run on a subset of the clusters; for example, you might
want profiling to run only on the clusters that represent your staging environment. You can assign a
`Module` selectively by giving it labels to match against `Cluster` object metadata.

Some configurations need to be tailored to the cluster before they are applied. For example, a
configuration that has to use a different database depending on the availability zone. <!-- FIXME
better example! --> You can customise a configuration for each cluster with values taken from the
cluster definition, or another object like a `ConfigMap`.

Fleeet co-ordinates in a central cluster, but tries to avoid making this a [single point of
failure][].

[single point of failure]: https://en.wikipedia.org/wiki/Single_point_of_failure

## How to install it

See the [installation instructions][].

## How to use it

See the [API reference][], or follow the [getting started][] walkthrough.

[controller]: https://kubernetes.io/docs/concepts/architecture/controller/
[Kubernetes' API extensions]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[installation instructions]: ./install.md
[API reference]: ./api.md
[getting started]: ./walkthrough.md

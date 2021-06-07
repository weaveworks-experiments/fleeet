<!-- -*- fill-column: 100 -*- -->
# Other designs

## Rancher Fleet

In [Rancher Fleet][rancher-fleet] you register a downstream cluster by:

 - creating a token in the upstream cluster
 - running a Helm chart in the downstream cluster, supplying the token

The agent in the downstream connects to the upstream API server, and gets instructions from objects
in a namespace dedicated to the cluster.

A GitRepo defines (paths in) a git repository to treat as configuration. The git repo is analysed to
figure out how to package it as Bundles. A Bundle has a specification for how it gets rolled out,
and how it is to be assigned to clusters, and how it is customised for each cluster. These can come
from the GitRepo or from a file representing the bundle, in the git repo.

A manager process in the upstream cluster calculates a BundleDeployment specialising each assigned
Bundle for each cluster, and makes the content available through the Kubernetes API for the
downstream agent to download.

Upsides:

 - using the Kubernetes API as the channel for control and distribution is elegant, since it means
   you don't need to figure out anything special and can rely on Kubernetes access control
 - it has a comprehensive system for customising configuration to particular clusters
 - similarly, a sophisticated system for rollouts

Downsides:

 - it's all-or-nothing, you can't use it with any other tooling, customise how it works, or
   realistically port your git repo to another tool
 - using the Kubernetes API is a scaling challenge
 - lots of ways of doing things = complexity for the user
 - sync or runtime dependence is not supported

## ArgoCD ApplicationSet

The [ApplicationSet controller](https://argocd-applicationset.readthedocs.io/en/stable/) is an Argo
labs project which addresses some fleet concerns. The design centre is a mechanism for generating
Application definitions from a set of generators. Each generator yields either a list of parameter
replacements (e.g., cluster names), or Application templates (e.g., from directories in a git
repository), and the controller effectively calculates the product `parameters x templates` to
produce Application definitions.

This is an elegant mechanism, and I think easy to grasp as a user. It is an admin-centric model,
because the entry point is an object which assumes a view of all clusters and all configurations.

Advantages:

 - it is obvious to the user what is happening with the parameter substitution, because it's all in
   one place
 - it feels like a natural extension to the Application abstraction

Disadvantages:

 - the result is multiplied out on the management cluster (vs the law of parsimony).
 - since the aggregate object gives an intension for both modules (units to be synced) and clusters,
   neither of these are represented individually -- you don't have a view of a module as rolled out,
   or all things on a cluster (though you can construct at least the latter)

In principle, you could replicate the mechanism for use with GOTK, and it would have both the
advantages and the flaws. It might feel a little less like a natural extension to the syncs in GOTK,
because they come in two disjoint varieties (Kustomizations and HelmReleases) which don't
interoperate.

What would it look like to try and reproduce the advantages, while mitigating the disadvantages?
Keeping separate units of sync (modules) and combining into an aggregate for a particular cluster
mitigates both the disadvantages, since it keeps the number of objects down to O(clusters), and lets
you see the rollout status of a module. To retain the transparency of interpolation advantage,
parameters need to be specified at the point of access, i.e., modules, and work independently (no
interaction with things specified elsewhere).

## Starling

[Starling][starling] is my testing ground for some ideas around fleets.

In the original design, there are two primitives: Sync and SyncGroup. A sync takes a URL to download
a package (e.g., the URL for a release archive on GitHub) and optionally a target cluster (as a
secret containing a kubeconfig). A SyncGroup creates Syncs for Cluster API Cluster objects that
match a label selector.

Syncs can [depend on each other][starling-deps], giving a readiness level (as defined by kstatus)
that must be reached by the dependency before the dependent is synced.

SyncGroups roll changes out by updating only some of their Sync objects at a time.

Sync and SyncGroup are a minimal set of primitives for managing arbitrary numbers of clusters. You
get:

 - syncs assigned to clusters according to rules (SyncGroup)
 - some respect of dependence
 - rolling updates

Aside from not being intended for production use, there are these downsides:

 - it is left to the user to arrange them to the desired effect, e.g., to figure out how to register
   a downstream cluster
 - since Syncs are applied directly from the upstream cluster, it is a single point of failure. If
   it becomes unavailable, no syncing happens

In [a variant on the design][starling-alt], Syncs only act locally, and there are additional
primitives RemoteSync and ProxySync for applying to other clusters. These differ on how they apply
configuration -- the RemoteSync applies directly to the downstream cluster, and the ProxySync
creates a Sync in the downstream cluster and monitors it.

This is mainly to address the problem with the management cluster being a single-point of
failure. RemoteSyncs can be used for bootstrapping (getting the base RBAC and controllers on to the
downstream cluster), then ProxySyncs used. If the management cluster is unavailable, the syncs
downstream will continue to run at least.

The main advance is the ProxySync; the other downsides and gaps remain.

## Flux v2 / GitOps Toolkit

[GitOps Toolkit][gotk] is a suite of Kubernetes API extensions and accompanying controllers that
each do one of these jobs:

 - apply configuration to the cluster (kustomize-controller, helm-controller)
 - proxy sources of configuration into the cluster (source-controller)
 - integrate with the world outside the cluster (notification-controller,
   image-reflector-controller)
 - make automated updates to a git repository (image-automation-controller)

Flux v2 ties them into a known-good configuration and gives you a command-line interface for
creating and interacting with the combined API.

GitOps Toolkit does not support fleets, because it lacks APIs/controllers that work at the level of
arbitrary numbers of clusters. For example, there is no mechanism for assigning configurations to
clusters. But it does have some integration points that make it useful as a base for constructing
fleet kit:

 - Kustomizations can have variables interpolated with envsubst syntax (so a particular
   configuration could be specialised to a cluster)
 - Kustomizations and HelmReleases can target another cluster by referring to a kubeconfig
   secret. This could be useful for bootstrapping (it's similar to Starling's RemoteSync)
 - there is an [elegant multi-tenancy model][gotk-tenancy] in the works

Here are some things that would need to be worked into the design, or worked around:

 - to do rollouts in a fleet, you want fine control over the version of configuration for each
   package you want to roll out, in each cluster. GOTK is intended to be driven the other way: it's
   a soft assumption that packages will tend to depend on a common set of sources, and all packages
   should be updated whenever their source has a new revision. In a fleet setting, you want the
   policy to be calculated upstream and enacted downstream; GOTK puts them together.
 - the dependence mechanism is limited:
   - it only operates within a type, e.g., Kustomization->Kustomization but not
     Kustomization->HelmRelease
   - it's not transitive over syncs, so if you have an indirection you lose the relation; e.g., if I
     have a Kustomization that depends on a webhook being installed, and the configuration it
     applies includes Kustomization objects, those won't also depend on the webhook. This makes it
     hard to enforce dependence since you can always escape it with an indirection, and the other
     aspects of the design (like only operating within types) encourage you to use indirections
   - you can only specify a dependence between one object and another, and only from the side of the
     dependent; so you can't express a relation like "all other syncs should depend on this one"


[starling]: https://github.com/squaremo/starling
[starling-deps]: https://github.com/squaremo/starling/blob/main/docs/rfc/0002-dependencies.md
[starling-alt]: https://github.com/squaremo/starling/blob/all-the-syncs/docs/rfc/0000-sync-primitives.md
[rancher-fleet]: https://fleet.rancher.io/
[gotk]: https://toolkit.fluxcd.io/components/
[gotk-tenancy]: https://github.com/fluxcd/flux2/pull/582

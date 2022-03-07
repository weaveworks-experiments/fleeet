<!-- -*- fill-column: 100 -*- -->
# RFC: Changes to accommodate rollouts

**Status: work in progress**

## Summary

This RFC describes changes to the design to facilitate "rollouts".

## Motivation and requirements

A core requirement of this project is handling automated rollout of new configurations. A rollout in
this context is the deployment of a versioned configuration to a set of clusters. There are several
strategies for continuous delivery in which automation helps:

 - blue/green deployments, in which new configuration is rolled out to a parallel set of clusters,
   then the live system flipped to that set (e.g., by changing traffic routing rules);
 - incremental rollouts e.g., canary deployments, in which a new configuration is deployed to a
   small set (possibly one) of clusters (possibly one) and if successful to the remainder;
 - deployment pipelines (e.g., dev to staging to prod) and adapting configurations to different
   environments.

For some if not all of the above strategies, integration points with external systems are desirable
-- for example, to observe the error rate of a service while rolling a new version of it out; or, to
effect a reroute of traffic. In addition, there will be scenarios with requirements not anticipated
here. For these reasons, the mechanism should in principle let third parties construct their own
rollout automation.

## Current design

Currently, a `Module` object's spec includes a selector for targeting clusters. The module
controller calculates the clusters selected, and adds or replaces a sync record, named for the
module, to a `RemoteAssemblage` object targeting the cluster:

```
       ┌───────────────────selects───────────────────────────────────────────┐
       │                                                                     │
┌──────┴──────┐                                                              │
│             │                                                              │
│ Module foo  ├─ ─ ─ ─┐                                                      │
│ v1          │                                                              │
│             │       │         ┌──────────────────┐       ┌─────────────┐   │
└─────────────┘                 │ RemoteAssemblage │       │             │◄──┘
                      └─ ─ ─ ──►│  foo: v1         ├───────►  Cluster A  │
                                │  bar: v1         │       │             │◄──┐
                      ┌─ ─ ─ ──►│                  │       └─────────────┘   │
                                └──────────────────┘                         │
                      │                                                      │
                                                                             │
                      │         ┌──────────────────┐                         │
                                │ RemoteAssemblage │      ┌──────────────┐   │
                      │         │                  │      │              │   │
┌─────────────┐       ├─ ─ ─ ──►│  bar: v1         ├──────►  Cluster B   │   │
│             │                 │                  │      │              │◄──┤
│ Module bar  ├─ ─ ─ ─┘         └──────────────────┘      └──────────────┘   │
│ v1          │                                                              │
│             │                                                              │
└─────┬───────┘                                                              │
      │                                                                      │
      └────────────────────selects───────────────────────────────────────────┘
```

In the diagram above, the module `foo` selects only one of the clusters, while the module `bar`
selects both. Thus, `foo` appears in only one `RemoteAssemblage`, while `bar` appears in both. When
a module changes, it changes in every cluster selected:

```
       ┌───────────────────selects───────────────────────────────────────────┐
       │                                                                     │
┌──────┴──────┐                                                              │
│             │                                                              │
│ Module foo  ├─ ─ ─ ─┐                                                      │
│ v1          │                                                              │
│             │       │         ┌──────────────────┐       ┌─────────────┐   │
└─────────────┘                 │ RemoteAssemblage │       │             │◄──┘
                      └─ ─ ─ ──►│  foo: v1         ├───────►  Cluster A  │
                                │  bar: *v2*       │       │             │◄──┐
                      ┌─ ─ ─ ──►│                  │       └─────────────┘   │
                                └──────────────────┘                         │
                      │                                                      │
                                                                             │
                      │         ┌──────────────────┐                         │
                                │ RemoteAssemblage │      ┌──────────────┐   │
                      │         │                  │      │              │   │
┌─────────────┐       ├─ ─ ─ ──►│  bar: *v2*       ├──────►  Cluster B   │   │
│             │                 │                  │      │              │◄──┤
│ Module bar  ├─ ─ ─ ─┘         └──────────────────┘      └──────────────┘   │
│ *v2*        │                                                              │
│             │                                                              │
└─────┬───────┘                                                              │
      │                                                                      │
      └────────────────────selects───────────────────────────────────────────┘
```

(`bar` has changed to `v2`)

The mechanism for `BootstrapModule` is different. This shows the how `Module` and `BootstrapModule`
are handled differently:

```
                                                            │ │
             Control Plane cluster                          │ │      Leaf cluster
                                                            │ │
                                                            │ │                            ┌────────────┐
┌──────────────────┐                                        │ │                            │            │
│                  │                                        │ │                            │ Flux prims ├─┐
│     Module       ├─┐              ┌───────────────────┐   │ │     ┌───────────────┐      │            │ │
│                  │ │    aggregate │                   │   │ │     │               │      └─┬──────────┘ ├─┐
└─┬────────────────┘ ├─┐ ──────────►│ RemoteAssemblage  ├───┴─┴────►│  Assemblage   ├──────► │  ...       │ │
  │  ...             │ │            │                   │  proxy    │               │expand  └─┬──────────┘ │
  └─┬────────────────┘ │            └───────────────────┘   │ │     └───────────────┘          │  ...       │
    │  ...             │                                    │ │                                └────────────┘
    └──────────────────┘                                    │ │
                                                            │ │
                                                            │ │
                                                            │ │
                                    ┌────────────┐          │ │
                                    │            │          │ │
 ┌─────────────────┐                │ Flux prims ├─┐        │ │
 │                 │    expand      │            │ │        │ │
 │ BootstrapModule ├──────────────► └─┬──────────┘ ├─┐ ─────┴─┴────►
 │                 │                  │  ...       │ │     apply
 └─────────────────┘                  └─┬──────────┘ │      │ │
                                        │  ...       │      │ │
                                        └────────────┘      | |
```

Bootstrap modules do not need to be proxied, because they will expand to Flux primitives in the
control plane, to be applied remotely -- that is the essential difference between bootstrap modules
and modules.

## New design

Rollouts require a mechanism for atomically updating from the old module spec to the new module
spec, for a cluster at a time. There is already a place in which module assignments for a cluster
are effectively recorded: the `RemoteAssemblage` type. To implement rollouts, the module controller
can update assemblages incrementally.

### BootstrapModules and rollouts

As the previous section explained, there is no analogue to `RemoteAssemblage` for
`BootstrapModule`. This means that unlike modules, there is no place for a bootstrap module to be
updated incrementally. To fix this, a new type is introduced (and names shuffled to be more
appropriate):

```
                                                                  │ │
                   Control Plane cluster                          │ │      Leaf cluster
                                                                  │ │
                                                                  │ │                            ┌────────────┐
      ┌──────────────────┐                                        │ │                            │            │
      │                  │                                        │ │                            │ Flux prims ├─┐
      │     Module       ├─┐              ┌───────────────────┐   │ │     ┌───────────────┐      │            │ │
      │                  │ │   aggregate  │ ProxyAssemblage   │   │ │     │               │      └─┬──────────┘ ├─┐
      └─┬────────────────┘ ├─┐ ──────────►│ (was              ├───┴─┴────►│  Assemblage   ├──────► │  ...       │ │
        │  ...             │ │            │  RemoteAssemblage)│  proxy    │               │expand  └─┬──────────┘ │
        └─┬────────────────┘ │            └───────────────────┘   │ │     └───────────────┘          │  ...       │
          │  ...             │                                    │ │                                └────────────┘
          └──────────────────┘                                    │ │
                                                                  │ │
                                                                  │ │
 ┌─────────────────┐                  ┌──────────────────┐        │ │
 │                 │    aggregate     │                  │        │ │
 │ BootstrapModule ├─┐ ──────────────►│ RemoteAssemblage │        │ │
 │                 │ │                │ (new)            │        │ │
 └─┬───────────────┘ ├─┐              └──┬───────────────┘        │ │
   │  ...            │ │                 │                        │ │
   └─┬───────────────┘ │              expand                      │ │
     │  ...            │                 │    ┌────────────┐      │ │
     └─────────────────┘                 │    │            │      │ │
                                         └───►│ Flux prims ├─┐ ───┴─┴──────►
                                              │            │ │   apply
                                              └─┬──────────┘ ├─┐  │ │
                                                │  ...       │ │  │ │
                                                └─┬──────────┘ │  │ │
                                                  │  ...       │
                                                  └────────────┘
```

In tabular form:

| Type | Job |
|------|-----|
| Assemblage | Specify syncs to be expanded into Flux primitives, within a downstream |
| ProxyAssemblage | Maintain an Assemblage in a downstream, from upstream |
| RemoteAssemblage | Specify syncs to be expanded into Flux primitives targeting a downstream |

This makes bootstrap modules amenable to rollouts in the same way as modules.

### Rollout automation

Rollout automation works by replacing the sync records in assemblages. To do this incrementally, the
controller will usually need to check how many of the sync records have the most recent
specification, and have successfully been synced. The rollout strategy (and any accompanying
parameters) specifies when the controller can update more sync records.

#### Replacement rollout

In a replacement rollout, the module as given by a template is applied simultaneously to all
clusters matching a selector.

```yaml
apiVersion: fleet.squaremo.dev/v1alpha1
kind: Module
metadata:
  name: replace
  namespace: default
spec:
  selector:
    matchLabels: {env: dev}
  strategy: Replace
  template:
    spec:
      controlPlaneBindings: ...
      sync: ...
```

The implementation is roughly the following:

 - find or create an assemblage object for all clusters that match the selector
 - for each assemblage, make sure there is a sync record with the latest sync specification from
   this module.

This is the equivalent of `Module` behaviour in the current design.

#### Incremental rollout

An incremental rollout deploys changes gradually: when there is a change to the module template, the
controller picks a cluster to apply the new configuration to; if that is successful, it applies the
new version to all clusters.

```yaml
apiVersion: fleet.squaremo.dev/v1alpha1
kind: Module
metadata:
  name: staging-canary
  namespace: default
spec:
  selector:
    matchLabels: {env: staging}
  strategy: Incremental
  rollout:
    maxUnreadyClusters: 2
  template: # of a module
    spec:
      controlPlaneBindings: ...
      sync: ...
```

The algorithm is roughly this:

 - find or create an assemblage for each cluster that's selected
 - count the number of assemblages for which the sync record for this module have not successfully
   synced; if fewer than `maxUnreadyClusters`, pick one assemblage that has an out-of-date sync
   record and update it.

### Implementing third party rollout automation

Third parties can implement their own rollout automation by manipulating assemblages.

## Summary of changes proposed

 - `Module` and `BootstrapModule` gain rollout strategy fields, as in the examples given above. The
   module controller and bootstrap module controller implement the rollout algorithms.
 - The current `RemoteAssemblage` type is renamed to `ProxyAssemblage`
 - `RemoteAssemblage` now names a new type that represents the bootstrap module assignments for a
   cluster; the bootstrap module controller constructs the remote assemblages, and the remote
   assemblage controller expands syncs into Flux primitives
 - Expanding control-plane bindings is now done by the assemblage controllers, so that third party
   automation doesn't need to reimplement it.

## Alternatives considered

TODO expand on these.

 - Rollouts as a layer on modules (against: atomicity of changes, and more objects)
 - Referring to modules in RemoteAssemblage (against: then you'd have to build other rollout
   automation from scratch)
 - Represent module assignment in another object, so it's not conflated with multiplexing (against:
   another object with the same information; ownership v GC)

## Open questions and suggestions

 * Could the module spec look more like the generator syntax of KustomizationSet?

 * Is there a better name than `Module`? (surely!)
   - `ModuleSet` as in, a dynamic set of modules
   - `SyncSet` as in set of syncs
   - `Assignment` as in assigment of configuration to clusters
   - `Rolloutment` as in silly mix of Rollout and Deployment
   - perhaps `Module` should be renamed `Rollout`, since it describes to the application of a
     configuration at a version (though not the target)

 * People may wish to see the history of a rollout -- When did the rollout start? How did the last
   rollout go? One possibility is to keep a history in the status, like Deployment does.

 * Alternative: have Rollout as a verb, and treat each as a one-off process. (Trouble: this sits
   less well with Kubernetes' model, as it's an imperative, rather than a declared, desired state).

 * The whole design to date requires all objects to be in the same namespace, including clusters and
   their accompanying secrets. This is OK if there's a single team (or person!) that is running
   everything; but what if you want to divide responsibility between e.g., platform admins who can
   create clusters, and application developers who can roll their configuration out, but not access
   clusters arbitrarily.

 * How does recovery work? You need the ability to:
  - pull the handbrake: stop where you are
  - pin a problematic cluster where it is, and continue
  - roll back to the previous state safely
  - roll forward
  - Assumption: you are never going to rebuild the whole fleet to the state that it's in. But you might want to recover:
    - the control plane
    - individual clusters to a "known good" state (i.e., possibly before you started a rollout)

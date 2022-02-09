<!-- -*- fill-column: 100 -*- -->
# RFC: Changes to accommodate rollouts

**Status: work in progress**

## Summary

This RFC describes a mechanism for controlling rollouts, and how to build specific rollout
primitives upon that mechanism.

## Motivation and requirements

A core requirement of this project is handling automated rollout of new configurations. A rollout in
this context is the deployment of a versioned configuration to a set of clusters. There are several
strategies for continuous delivery in which rollout automation helps:

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

The current design makes "third party" rollouts tricky, because configurations are assigned to
clusters using labels and label selectors, and to dynamically adjust which clusters get which
versions of a configuration you would need to manipulate those labels and selectors.

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

It would be possible in principle to add a rollout specification into the `Module` type, and
implement it directly in the module controller by changing only some `RemoteAssemblage` objects at a
time. However, this would make it harder to admit third party rollout logic, because it would either
have to ignore (or replace) the module layer, or fight the built-in logic. This suggests that the
mechanism for rollouts should be based on replacing one module (assignment to a cluster) for
another, rather than updating a module.

Abstractly, to make a change incrementally you would have to create a module with the new version,
and change the assignment so that the new version is assigned to one selected cluster, and the old
version to the remainder of the selected clusters. But any scheme which switches one module for
another exposes a flaw in the current design: swapping one module for another is not atomic, as the
next section explains.

### Keeping it atomic

`BootstrapModule` and `Module` are expanded to `Kustomization` and `GitRepository` objects. If a
kustomization changes name, to Flux this looks like a kustomization being removed then another
kustomization being added (or a kustomization being added then a kustomization being removed) --
which means it could remove the contents of a sync before restoring them, e.g., deleting a
`Deployment` then creating it again. Since, ultimately, the kustomizations are named for the module
that caused their creation, moving from one module to another will lead to a name change, and a
break in continuity.

A similar situation arises in the module controller during an incremental rollout. Since it sees one
module at a time, when a module is replaced with another it will either remove the first then add
the second, or add the second then remove the first.

To avoid breaking continuity, the design here needs to ensure two things when moving a cluster from
one version of a configuration to another:

 1. the resulting Kustomization object is named the same; and,
 2. the change is effected in one transaction (i.e., by updating a single object).

A simple way to ensure 1.) is for the name of the Kustomization to come from the single object that
owns the modules representing the configuration versions (which is probably the object representing
the rollout). The effect will be that moving from one to the other will update the explicitly-named
Kustomization object rather than removing one and creating another.

For 2.), it's necessary to rejig the API so that assigned modules are evaluated a cluster at a time
rather than a module at a time. By doing so, when the module assignments to a cluster change, the
entire set of modules will be calculated in one go.

## New design

To recap the problems identified in the previous sections: rollouts require a mechanism for
atomically replacing a module assignment with another. In other words, the "increment" in an
incremental rollout should be a pointer flip, from one module to another, for a cluster at a time.

There is already a place in which module assignments for a cluster are effectively recorded: the
`Assemblage` and `RemoteAssemblage` types. At present, these objects are created and updated when a
module selection changes:

    module assignment (selection) change -> assemblage update

By reversing this, it's possible to satisfy the requirement from above:

    assemblage update -> module assignment change

That is: an assemblage _represents_ the module assignments for a cluster, rather than _reflecting_
assignments made elsewhere. To change the assignment, you change the assemblage.

Making the assignment explicit at this level creates an affordance for controlling rollouts. For
example, you can effect a simple incremental rollout by creating a module with the new configuration
version, then changing assemblage objects from the existing module to the new module.

The owner record is a weak reference -- if a cluster is deleted, its relation to modules goes with
it. The module controller will need to clean up owner records for modules which are deleted.

### BootstrapModules and rollouts

The previous sections have not accounted for BootstrapModules, and elided between remote assemblages
and assemblages in general, which was a minor sleight of hand. At present, (non-remote) assemblages
are objects created in leaf clusters by the remote assemblage controller, which are expanded locally
into Flux primitives (kustomizations and git repositories). There is no analogue to RemoteAssemblage
in which the `BootstrapModule` assignments are recorded.

This shows the current design:

```

                                                            │ │
             Control Plane cluster                          │ │      Leaf cluster
                                                            │ │
                                                            │ │                            ┌────────────┐
┌──────────────────┐                                        │ │                            │            │
│                  │                                        │ │                            │ Flux prims ├─┐
│     Module       ├─┐              ┌───────────────────┐   │ │     ┌───────────────┐      │            │ │
│                  │ │              │                   │   │ │     │               │      └─┬──────────┘ ├─┐
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

Bootstrap modules do not need to be proxied, because they will expand to Flux primitives that are
applied remotely -- that is the essential difference between BootstrapModules and Modules. This
means that unlike modules, there is not a place for assignments to be updated explicitly. However:
notice the other thing that is different -- an assemblage in the leaf cluster reflects the
assignment of modules to that cluster. The same could be made the case for bootstrap modules:

```
                                                                 │ │
                  Control Plane cluster                          │ │      Leaf cluster
                                                                 │ │
                                                                 │ │                            ┌────────────┐
     ┌──────────────────┐                                        │ │                            │            │
     │                  │                                        │ │                            │ Flux prims ├─┐
     │     Module       ├─┐              ┌───────────────────┐   │ │     ┌───────────────┐      │            │ │
     │                  │ │   aggregate  │                   │   │ │     │               │      └─┬──────────┘ ├─┐
     └─┬────────────────┘ ├─┐ ──────────►│ RemoteAssemblage  ├───┴─┴────►│  Assemblage   ├──────► │  ...       │ │
       │  ...             │ │            │                   │  proxy    │               │expand  └─┬──────────┘ │
       └─┬────────────────┘ │            └───────────────────┘   │ │     └───────────────┘          │  ...       │
         │  ...             │                                    │ │                                └────────────┘
         └──────────────────┘                                    │ │
                                                                 │ │
                                                                 │ │
┌─────────────────┐       ┌────────────────┐                     │ │
│                 │aggre- │                │                     │ │
│ BootstrapModule ├──────►│   Assemblage   │                     │ │
│                 │gate   │                │ ┌────────────┐      │ │
└─────────────────┘       └──────┬─────────┘ │            │      │ │
                                 │           │ Flux prims ├─┐ ───┴─┴──────►
                                 │           │            │ │   apply
                                 │ expand    └─┬──────────┘ ├─┐  │ │
                                 └──────────►  │  ...       │ │  │ │
                                               └─┬──────────┘ │  | |
                                                 │  ...       │  | |
                                                 └────────────┘  | |
```

This makes bootstrap modules amenable to rollouts in the same way as modules.

### Rollouts

It is now possible to explicitly assign modules to clusters -- but if modules no longer select
clusters, what manages the assignments?

To control the assignment of modules to clusters, there is a new type `Rollout`. A rollout
represents the dynamic assignment of a configuration to a set of clusters, as a selector, and a
strategy for effecting changes. To start with, there are two strategies, described in the following
sections. This is an extension point for future work, since third parties can use the same mechanism
to construct their own rollout automation.

#### Rollout mechanism

Rollout automation operates by replacing the module assignments in clusters, incrementally.

TODO: more explanation, and diagram.

#### Replacement rollout

In a replacement rollout, the module as given by a template is applied simultaneously to all
clusters matching a selector.

```yaml
apiVersion: fleet.squaremo.dev/v1alpha1
kind: Rollout
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

The implementation is to maintain a module with the details given in the template, and assign it to
all remote assemblage objects that match the selector. When the template is changed, the module is
updated in place.

This is the analogue of a `Module` from before this RFC.

#### Incremental rollout

An incremental rollout deploys changes gradually: when there is a change to the module template, the
controller picks a cluster to apply the new configuration to; if that is successful, it applies the
new version to all clusters.

```yaml
apiVersion: fleet.squaremo.dev/v1alpha1
kind: Rollout
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

The implementation is to ensure there is a module with the template given, then incrementally move
assignments to this module according to the parameters given -- e.g., if there are clusters that
need updating, and fewer than `minUnreadyClusters`, update the assignment for another cluster.

# Open questions and suggestions

 * Should it be possible to use this with bootstrap modules? I don't think the logic of rollouts
   changes.

 * Could it look more like the generator syntax of KustomizationSet?

 * Is there a better name than `Rollout`? (which is a verb as well as a noun, so suggests an
   imperative).
   - `ModuleSet` as in, a dynamic set of modules (if I keep "Module")
   - `SyncSet` as in set of syncs
   - `Assignment` as in assigment of configuration to clusters
   - `Rolloutment` as in silly mix of ROllout and Deployment

 * People may wish to see the history of a rollout -- When did the rollout start? How did the last
   rollout go? One possibility is to keep a history in the status, like Deployment does.

 * Alternative: have Rollout as a verb, and treat each as a one-off process. (Trouble: this sits
   less well with Kubernetes' model, as it's an imperative, rather than a declared, desired state).

 * The whole design to date requires all objects to be in the same namespace, including clusters and
   their accompanying secrets. This is OK if there's a single team (or person!) that is running
   everything; but what if you want to divide responsibility between e.g., platform admins who can
   create clusters, and application developers who can roll their configuration out, but not access
   clusters arbitrarily.

<!-- -*- fill-column: 100 -*- -->
# RFC: Changes to accommodate rollouts

**Status: work in progress**

## Summary

This RFC describes a mechanism for controlling rollouts, and how to build specific rollout
primitives upon that mechanism.

## Motivation and requirements

A core requirement of this project is handling automated rollout of new configurations. There are
several strategies for continuous delivery in which rollout automation helps:

 - blue/green deployments, in which new configuration is rolled out to a parallel set of clusters,
   then the live system flipped to that set (e.g., by changing traffic routing rules);
 - incremental rollouts e.g., canary deployments, in which a new configuration is deployed to a
   small set (possibly one) of clusters (possibly one) and if successful to the remainder;
 - deployment pipelines (e.g., dev to staging to prod) and adapting configurations to different
   environments.

The current design makes rollouts tricky, because configurations are assigned to clusters using
labels and label selectors, and to dynamically adjust which clusters get which versions of a
configuration you would need to manipulate those labels and selectors. Another mechanism is needed.

For some of the above use cases, integration points with external systems are desirable -- for
example, to observe the error rate of a service while rolling a new version of it out; or, to effect
a reroute of traffic. In addition, there will be use cases with requirements not anticipated
here. For these reasons, the mechanism should in principle let third parties write their own rollout
automation.

## Module to cluster assignment

Currently, a `Module` object's spec includes a selector for targeting clusters. The module
controller calculates the clusters selected, and applies the sync defined in the module to each one:

```
                 ┌────────────┐
                 │            │
                 │   Module   ├─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
                 │            │
                 └──────┬─────┘                          │
                        │
        ┌────────────selects────────────┐                │
        │               │               │
   ┌────▼──────┐  ┌─────▼─────┐  ┌──────▼────┐           │
   │           │  │           │  │           │         assign
   │  Cluster  │  │  Cluster  │  │  Cluster  │          sync
   │           │  │           │  │           │           |
   └─────▲─────┘  └─────▲─────┘  └─────▲─────┘
         │              │              │                 |

         └ ─ ─ ─ ─ ─ ─ ─┴─ ─ ─ ─ ─ ─ ─ ┴ ─ ─ ─ ─ ─ ─ ─ ─ ┘
```

In the new design, Modules do not contain a specification for which clusters they should be applied
to. Instead, assignment is represented by an owner record in a cluster, pointing at the assigned
module, and it is assumed that some other layer will arrange these records. The module controller
then makes sure each cluster has the assigned modules applied to it.

```
                    ┌──────────┐
                    │          │
                    │  Module  ├ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
                    │          │
                    └────▲─────┘                           │
                         │
       ┌──────────────belong─────────────┐                 │
       │                 │               │
 ┌─────┴─────┐    ┌──────┴────┐    ┌─────┴─────┐           │
 │           │    │           │    │           │        assign
 │  Cluster  │    │  Cluster  │    │  Cluster  │         sync
 │           │    │           │    │           │           │
 └─────▲─────┘    └──────▲────┘    └──────▲────┘
       │                 │                │                │
       └ ─ ─ ─ ─ ─ ─ ─ ─ ┴─ ─ ─ ─ ─ ─ ─ ─ ┴ ─ ─ ─ ─ ─ ─ ─ ─┘
```

Making the assignment explicit at this level creates an affordance for controlling rollouts. For
example, you can effect a simple incremental rollout by creating a module with the new configuration
version, then moving clusters from the existing module to the new module.

The owner record is a weak reference -- if a cluster is deleted, its relation to modules goes with
it. The module controller will need to clean up owner records for modules which are deleted.

## Rollouts

To control the assignment of modules to clusters, there is a new type `Rollout`. A rollout
represents the dynamic assignment of a configuration to a set of clusters, as a selector, and a
strategy for effecting changes. To start with, there are two strategies, described in the following
sections. This is an extension point for future work.

### Rollout mechanism

TODO: explanation including a diagram.

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

The implementation is to maintain a module with the details given in the template, and give all the
matching clusters an owner record to it. When the template is changed, the module is updated in
place.

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

#### Keeping it atomic

`BootstrapModule` and `Module` are expanded to `Kustomization` and `GitRepository` objects. If a
Kustomization object changes name, to Flux this looks like a Kustomization object being removed then
another Kustomization object being added (or an object being added then an object being removed) --
which means it could remove the contents of a sync before restoring them, e.g., deleting a
Deployment then creating it again. Since, ultimately, the Kustomization objects are named for the
module that caused their creation, moving from one module to another will lead to a name change, and
a break in continuity.

A similar situation arises in the module controller during a rollout. Since it sees one module at a
time, when a module is replaced with another it will either remove the first then add the second, or
add the second then remove the first.

To avoid breaking continuity, the design here needs to ensure two things when moving a cluster from
one version of a configuration to another:

 1. the resulting Kustomization object is named the same; and,
 2. the change is effected in one transaction (i.e., by updating a single object).

A simple, though not foolproof, way to ensure 1.) is to supply the name to use with the module. For
modules that represent the same configuration at two different versions, the effect will be that
moving from one to the other will update the explicitly-named Kustomization object rather than
removing one and creating another.

This has the downside that it doesn't make sense outside of rollouts, which can ensure that only one
such module applies to any cluster at a time; it's possible to end up with a non-deterministic
configuration. This could be guarded by the controller, though it would be preferable to simply
disallow it in the model, if that were possible.

For 2.), it's necessary to rewrite the module controller so that it works a cluster at a time rather
than a module at a time. By doing so, when the module assignments to a cluster change, the entire
set of modules will be calculated in one go.

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

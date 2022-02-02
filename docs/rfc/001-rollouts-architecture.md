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
   environments

The current design makes this tricky, because configurations are assigned to clusters using labels
and label selectors, and dynamically adjusting which clusters get new versions of a configuration
would mean manipulating the labels. Another mechanism is needed.

For some of the above use cases, integration points with external systems are desirable -- for
example, to observe the error rate of a service while rolling a new version of it out; or, to effect
a reroute of traffic. In addition, there will be use cases with requirements not anticipated
here. For these reasons, the mechanism should in principle let third parties write their own rollout
automation.

## Partition mechanism

Currently, a `Module` object's spec includes a selector for targeting clusters. The module
controller calculates the clusters selected, and applies the sync defined in the module to each one.

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

In the new design, a `Module` object refers to a `Partition` object (henceforth "partition"), rather
than selecting clusters directly. The module controller then makes sure all clusters belonging to
the partition have the module applied to them.

```
                    ┌──────────┐
                    │          │
                    │  Module  ├ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
                    │          │
                    └─────┬────┘                           │
                          │
                          │                                │
                        refer
                          │                                │
                          │
                  ┌───────▼─────┐                          │
                  │             │
                  │  Partition  │                          │
                  │             │
                  └──────▲──────┘                          │
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

This layer of indirection creates an affordance for controlling rollouts by manipulating which
clusters belong to which partition objects (henceforth: "partitions"). For example, you can effect a
simple incremental rollout by creating a fresh partition and a module with the new configuration
version, referring to the new partition; then, moving clusters from the existing partition to the
new partition.

The new `Partition` type represents a set of clusters. Rather than enumerating the clusters in a
Partition object (henceforth "partition"), clusters are owned by the partitions to which they
belong.

 1. This is a weak reference: if a cluster is deleted, so is its membership to any partitions. If a
partition is deleted, the finaliser should remove owner entries from clusters that have them (see
3.).
 2. Given a cluster, you can see the partitions to which it belongs; and,
 3. By keeping an index, you can easily query for the clusters belonging to a partition.

A new controller, the partition controller, runs the finaliser for partitions to make sure there are
no dangling owner references. Since partitions are inert -- they do not have their own behaviour --
there is nothing else for the controller to do.

## Rollouts

To assign a module to a set of clusters, it is now necessary to:

 - create a partition
 - make the clusters in the set belong to the partition
 - create a module that refers to the partition

To automate this, there is another new type `Rollout`. A rollout represents the dynamic assignment
of a configuration to a set of clusters, and a strategy for handling changes. To start with, there
are two strategies, described in the following sections. This is an extension point for future work.

### Replacement rollout

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

The implementation is to maintain a partition containing all the matching clusters, and a module
pointed at it. The module controller takes care of applying the configuration. (This is the analogue
of a Module from before this RFC.)

### Incremental rollout

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

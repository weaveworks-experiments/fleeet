<!-- -*- fill-column: 100 -*- -->
# Modules

A Module defines a logical unit of synchronisation.

The responsibilities of a Module are:

 - a reference to an exactly versioned piece of configuration
 - the locus for assigning configuration to be synced to a cluster, according to rules
 - a unit of "roll out"; e.g., if you have an app deployed to many clusters, and you want to roll
   out a new version, the Module get the new version and specifies the strategy for rolling it out

**Why not have more than one sync def in a module?**

Technically this is possible, of course. What is to be gained? It would be possible to have internal
structure to a module -- e.g., dependencies between parts. But I think you would still want to deal
with single, versioned piece of configuration when rolling out, so if modules are what you roll out,
arguably that structure (and complexity) belongs outside modules.

**Why can you specify a revision or tag in a module, but not a branch?**

The purpose of a Module is to define an exact target state for the module. If you gave a branch,
that would not be exact -- it would imply following the branch HEAD. That means the recovery state
of your fleet is not well defined, so it is more difficult to roll back, or to audit, past
configurations.

Admittedly, tags are a grey area, since they can be moved. Allowing tags is a concession to
practicality for people who want to release configurations via e.g., semver tags. It's not worth
having another layer just to translate a tag into a revision.

## Roll out

Modules have a similar idea of "rollout" to Deployments. Unlike Deployments, which create pods to
roll out a new version, they cannot create clusters -- they have to work within the clusters that
are assigned.

The simplest strategy for rollout is to simply give all assigned clusters the new version. The next
simplest is to update one cluster at a time, and wait until it is ready before updating the next.

The trickier questions come when you consider what to do if the assigned clusters are still in a
rollout when you begin another, i.e., there is already at least one cluster in updating
state. Naively, you might simply wait until there are no clusters in updating state.

## Effect of Modules in the assemblage layer

Each module that applies to a cluster is added to a RemoteAssemblage for that cluster.

This diagram shows three modules assigned to a cluster, compiled to a RemoteAssemblage. The
assemblage layer creates an Assemblage in the downstream cluster, where it is decomposed into
GitRepository and Kustomization objects.

```
           Cluster 1                    :
              │                         :
Module A ─────┤        UPSTREAM         :      DOWNSTREAM
              │                         :
Module B ─────┤                         :
              │                         :
Module C ─────┤                         :
              └────► RemoteAssemblage ──────► Assemblage
                       - Sync A         :          │
                       - Sync B         :          ├──────► GitRepository...
                       - Sync C         :          │
                                        :          └──────► Kustomization...
```

### Bootstrap modules

Each downstream cluster needs to be prepared for syncing, by installing the GitOps Toolkit
definitions (CRDs and controller deployments) and the Assemblage definitions. Naturally, the
configuration of each of these -- or possibly both at once -- is referenced in a Module. However,
since they cannot assume any existing definitions or processes on the downstream cluster, they must
be applied from the upstream cluster.

In other words, instead of being multiplexed through the assemblage machinery, a `BootstrapModule`
is decomposed into a GitRepository, and a Kustomization which will be applied directly to each
cluster to which it is assigned.

There are at least two possible designs for this requirement:

 - keep syncing directly to the downstream cluster
 - sync directly initially, but create objects such that the downstream cluster will take over
   syncing itself

In the latter design, the configuration being synced would include the sync objects defining their
own sync (i.e., like Flux bootstrap works).

**Why not a BootstrapAssemblage?**

Because the idea is you can roll out new versions of a configuration in a BootstrapModule, as with a
Module, and assign to different clusters (though I would expect typically you'd assign bootstrap
modules to all clusters). Those are things that come with modules rather than assemblages.

**Why a different type rather than a field in Module?**

It does a different thing, so deserves to be a different type. Being a distinct API makes it much
easier to differentiate in RBAC rules.

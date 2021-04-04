<!-- -*- fill-column: 100 -*- -->
# Modules

A Module defines a logical unit of synchronisation.

The responsibilities of a Module are:

 - a reference to an exactly versioned piece of configuration
 - the locus for assigning configuration to be synced to a cluster, according to rules
 - a unit of "roll out"; e.g., if you have an app deployed to many clusters, and you want to roll
   out a new version, the Module get the new version and specifies the strategy for rolling it out

## Roll out

Modules have a similar idea of "rollout" to Deployments. Unlike Deployments, which create pods to
roll out a new version, they cannot create clusters -- they have to work within the clusters that
are assigned.

The simplest strategy for rollout is to simply give all assigned clusters the new version. The next
simplest is to update one cluster at a time, and wait until it is ready before updating the next.

The trickier questions come when you consider what to do if the assigned clusters are still in a
rollout when you begin another, i.e., there is already at least one cluster in updating
state. Naively, you might simply wait until there are no clusters in updating state.

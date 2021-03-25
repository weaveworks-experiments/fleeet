<!-- -*- fill-column: 100 -*- -->
# Modules

A Module defines a logical unit of synchronisation.

The responsibilities of a Module are:

 - a reference to an exactly versioned piece of configuration
 - a unit of "roll out"; e.g., if you have an app deployed to many clusters, and you want to roll
   out a new version, the Module get the new version and specifies the strategy for rolling it out
 - an object that is assigned to be synced to a cluster, according to rules

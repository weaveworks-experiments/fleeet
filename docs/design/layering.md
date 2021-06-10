<!-- -*- fill-column: 100 -*- -->
# Layering

**Status: fairly solid**

There are three layers:

 - the assemblage layer, which operates in workload clusters;
 - the remote layer, which communicates between workload clusters the control plane;
 - the module layer, which defines in aggregate what to deploy to workload clusters

The [**assemblage layer**][assemblages] runs in workload clusters, and gives specific, local
instructions for syncing. A remote assemblage in the remote layer (in the control plane) is a proxy
for an assemblage in a workload cluster. In an individual workload cluster, an assemblage is
decomposed into GitOps Toolkit primitives (e.g., GitRepository and Kustomization objects).

The assemblage layer exists to limit the number and kind of resources that the control plane cluster
must maintain and observe in the downstream cluster.

The [**remote layer**][remote-assemblages] represents the assignment of assemblages to clusters,
including any specialisation. Its purpose is to be the locus for communication between the control
plane and workload clusters, providing a proxy for each assemblage assigned to a workload
cluster. When a new set of syncs is calculated for the cluster, this is conveyed to the workload
cluster via the remote layer; and when the workload syncs succeed or fail, this is reflected
upstream in the remote layer.

The [**module layer**][modules] defines how configurations are to be composed and assigned to
workload clusters. New versions of modules are rolled out to all assigned clusters, as specified by
the module.

The module layer serves two purposes:

 - to define which configurations should be running in which clusters; and,
 - to manage rollouts of new configuration

[assemblages]: ./assemblages.md
[remote assemblages]: ./assemblages.md#remote-assemblages
[modules]: ./modules.md

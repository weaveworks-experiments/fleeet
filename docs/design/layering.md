<!-- -*- fill-column: 100 -*- -->
# Layering

There are three layers:

 - the [assemblage layer][./assemblage.md]
 - the [control layer][./control.md]
 - the [module layer][./module.md]

The **assemblage layer** runs in downstream clusters, and gives specific, local instructions for
syncing. A remote assemblage in the control layer is a proxy for an assemblage in a downstream
cluster. In an individual, downstream cluster, an assemblage is decomposed into GitOps Toolkit
primitives (e.g., GitRepository and Kustomization objects).

The assemblage layer exists to limit the number and kind of resources that the upstream cluster must
maintain and observe in the downstream cluster.

The **control layer** represents the assignment of assemblages to clusters, including any
specialisation. Its purpose is to be the locus for communication between the control place and
downstream clusters, providing a proxy for each assemblage assigned to a downstream cluster. When a
new set of syncs is calculated for the cluster, this is conveyed to the downstream cluster via the
control layer; and when the downstream syncs succeed or fail, this is reflected upstream in the
control layer.

The module layer defines how configurations are to be composed and assigned to downstream
clusters. New versions of modules are rolled out to all assignments, as specified by the module.

The module layer serves two purposes:

 - to define which configurations should be running in which clusters; and,
 - to manage rollouts of new configuration

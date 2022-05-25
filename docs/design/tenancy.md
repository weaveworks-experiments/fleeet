<!-- -*- fill-column: 100 -*- -->
# Reified tenancy

**status: freestyling**

Idea: represent tenancy in the object model. A tenancy is

 - abstractly, the grant of space to run things;
 - concretely, authorisation to run in certain namespaces on certain clusters

This could solve several problems:

 - cluster objects (under control of the platform team) will probably live in their own namespace,
   and modules etc. (under control of their respective application teams) will live in various
   namespaces.

 - once you have multiple users, as you surely will with a fleet, you want to tailor the access each
   has to clusters

 - this gives a lowest-common denominator way to represent clusters that don't have to come from
   Cluster API

 - if you want to restrict what users of a "single pane of glass" UI can see, this could give you
   the access control machinery on which to implement that.

## Design sketch

`Remote`: an object that points at a kubeconfig secret. This is an output for controllers (e.g., a
controller that creates a Remote for every Cluster API Cluster), and a target for things that select
clusters.

You can just create a Remote object, if you want to enrol a cluster manually.

`Tenant`: defines access to clusters (by selection), namespaces in the clusters, etc.

The tenancy controller expands Tenant records by:

 - creating a service account and RBAC objects in each remote cluster
 - constructing a kubeconfig secret pointing at the remote cluster, and using the service account
 - creating a Remote object refering to the kubeconfig secret.

## Implementation notes

 - minimal integration point is: create a service account for the tenant, and let the user bind
   roles and cluster roles to it in their own time
 - should work with, but not require, hierarchical namespaces

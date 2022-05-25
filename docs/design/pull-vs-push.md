<!-- -*- fill-column: 100 -*- -->
# Inversion of control (pull vs push)

**Status: figuring out**

In some scenarios you may require that connections are made only from downstream clusters to the
management cluster, and not the other way around. In this case, syncs are driven by the downstream
cluster observing objects in the management cluster, rather than by a controller in the management
cluster.

Tenancy complicates this a little, since a downstream cluster may support more than one tenant.

## Design sketch

Similar to the push model, the pull model protocol simply uses the Kubernetes API; so a pull
connection works by letting the downstream cluster connect to the management cluster, with a
carefully restricted set of permissions.

The following two sections give details and rationales for ...

 - how a downstream cluster is told how to connect to the management cluster (Enrolment)
 - how a downstream cluster obtains and processes instructions from the management cluster (Syncing)

This design concerns itself with processing Module objects; how (or whether) to deal with
BootstrapModule objects is an open question, discussed towards the end.

### Enrolment

Enrolment (registering a downstream cluster to sync with a management cluster) follows this scheme:

 1. provision the downstream cluster however you do that, and run the downstream pull agent
 2. create a capability for the downstream cluster to connect
 3. supply the capability to the downstream pull agent

#### Getting the downstream cluster ready

The downstream cluster needs to be running the Flux kustomize-controller and source-controller, and
the assemblage-controller and upstream-controller from this project. The user is responsible for
arranging this; tooling, examples, and pre-baked configs can of course help.

#### Creating a connection capability

Since the protocol works over the Kubernetes API, a capability means an API connection; i.e., a URL
and credentials. To be able to restrict access, a service account is created and given access to a
namespace representing the tenancy.

Question: can a system have both push and pull clusters? I don't see any technical reason why the
mechanisms cannot co-exist, so it is down to ergonomics -- does it make it difficult to use.

Question: at which granularity do service accounts get created? One per tenant? That would be
reasonably parsimonious, but does it make the system more vulnerable since you can get the service
account from any of the tenant clusters.

Decision: use a service account per tenant for now, as the simplest thing that works.

Question: how does the set of tenants for a downstream cluster get decided?

Decision: tenancy is determined by what the downstream cluster is told ("this cluster hosts tenant A
and tenant B").

An alternative is that tenancy is always determined in the management cluster, using its selection
mechanism, and each downstream cluster must figure out by looking in the management cluster which
tenants to service. This alternative involves more machinery, and means each downstream cluster must
be given permissions to be able to see which tenancies apply to it. For the selection mechanism to
work, each downstream cluster would also have to be represented by a Cluster object -- but while
it's possible to use the pull model with Cluster API, it's expected more to be used with other kinds
of cluster provisioning e.g., Terraform.

#### Connecting

The downstream cluster has to be told about the connection upstream. The information it needs can
all be contained in a kubeconfig:

 - the management cluster API endpoint,
 - credentials (a service account bearer token)
 - the namespace to use

This goes in a secret, and a custom resource `Upstream` (the name is just the first I thought of)
refers to that secret. A controller for Upstream objects sees the new object and connects, creating
a `Remote` (see [tenancy.md]()) in the management cluster to represent the downstream. Labels for the
Remote can be specified in the `Upstream` object.

### Syncing

Since it is represented as a Remote object, the downstream cluster can be selected by Modules and
participate in rollouts. However, the machinery has to differ a little since the control works the
other way around.

TODO: ASCII-art comparing the push and pull sync models.

Given the capability to connect to the management cluster (in an `Upstream`, the downstream agent
looks in the given management cluster namespace for ProxyAssemblage objects, and creates their
analogues in the local cluster. The (usual) Assemblage controller expands these into Flux
primitives, as before, and the agent pushes status updates back to the object in the management
cluster.

**Question**: how is the difference in behaviour between push and pull signalled to the management
cluster controllers?

It seems reasonable to start by marking a Remote as being push or pull (and referencing a kubeconfig
when push). Then there are a few alternatives:

 - the module controller understands when a Remote is pull, and creates a ProxyAssemblage marked as
   pull-based (possibly just by leaving out the reference to the kubeconfig); or,
 - ProxyAssemblages refer to remotes, and the proxy assemblage controller decides what to do based
   on how the remote is marked.
 - Use a different type to represent assemblages used for the pull model.

Decision: I would prefer not to let details of push/pull leak into the module layer, so I like the
idea of the ProxyAssemblage controller making this decision based on the remote in question; i.e.,
the second of the alternatives. The downside is that this drags Remote objects into the
ProxyAssemblage controller, rather than letting them just refer to a kubeconfig.

## Open questions

**Is there any meaning for BootstrapModules in the pull-based model?**

BootstrapModules would normally be applied directly from the management cluster to the downstream
cluster; but, in the case of pull remotes, connecting downstream is prohibited. RemoteAssemblages
could be acted upon by a pull cluster, but the effect would have to be the same as a (regular)
Module -- a Kustomization and GitRepository is created in the downstream cluster.

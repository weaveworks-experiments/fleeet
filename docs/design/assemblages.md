<!-- -*- fill-column: 100 -*- -->
# Assemblages

**Status: implemented**

An Assemblage defines a concrete plan for synchronisation, in aggregate. It is calculated from
higher level objects (usually in a control plane cluster), and applied by decomposing it into lower
level sync objects (usually in a worker cluster).

The responsibilities of an Assemblage are:

 - be a target of "compilation" for higher-level objects
 - act as a communication channel between control plane and worker clusters
 - define a self-contained, consistent, exactly-versioned composition of configuration

These things are ideally done in the layers _above_ Assemblages:

 - calculating which pieces of configuration go to which clusters
 - figuring out the exact versions for each piece
 - making sure all dependencies are satisfied

An assemblage is particular to a cluster, since it represents an exact constellation of syncs for
syncing.

Alternative: an assemblage has exact versions, but _another_ object ties it to a cluster. I might
want to have this if I think there are different flavours of tying to a cluster, e.g., sync this
directly vs. create this in the downstream and monitor it. The downside is that you now have more
objects, and it's not clear that deduplicating assemblages is worth the complexity.

**How to deal with secrets**

A remote assemblage does not come with secrets; it specifies how to obtain the secrets.

There will always be some groundwork on a downstream cluster to make secrets readable, whether it's
supplying a GPG key or assigning IAM roles.

## Open questions

Q: What is the simplest, _secure_ way to do this?

The simplest means of provisioning secrets is to create them directly in the downstream cluster. But
is this insecure, compared to e.g., getting them from the platform (KMS/AWS Secrets Manager)? Could
they be encrypted as created, and decrypted by the controller (is the extra layer needed? After all
the controller would need to be given a key, so ...).

Stefan says: I think you should consider multi-accounts instead of just multi-az, as most large AWS
clients can't run into a single account. Reaching the same KMS from multiple accounts is a different
story to multi-AZ.

**How do rollouts work?**

Rollouts are effected in the layer above, which decides which assemblages should be running which
exact versions of things.

**How does this interact with tenancy?**

I want tenancy to operate in the layer above and not be a specific concern on this layer. However:
if the tenancy model is that tenants get to introduce their own repo and syncs, where do you
calculate assemblages? Does the downstream cluster need to have RBAC, or is access control
considered settled before it reaches there? (Belt and braces answer: use different RBAC for tenant
assemblages).

---

Refs

KMS: https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/
(this encrypts data in etcd, not the secrets as accessed via the API)
and AWS Secrets Manager: https://aws.amazon.com/secrets-manager/


# Wishlist (principles and desiderata)

**Law of parsimony (of Kubernetes objects)**

Record only as much as necessary, especially in the control
plane. Avoid controllers which serve only to expand higher-level
definitions. Push actuation and aggregation downstream. (-> the
Assemblage)

**Trapeze acts are dangerous**

Avoid the situation in which lots of things from different places have
to line up for things to work (and where that can't be avoided, try to
make failure to line up visible, e.g., by shouting when there's a
dangling reference).

A specific variety of this situtation I would especially like to avoid
is having a sync relay: this git repo has a sync spec for this git
repo has a sync spec for this git repo has a sync spec... .

A remedy for this is to flatten the syncing topology into one
definition rather than distributing it through several places.

One reason GOTK causes this situation is that dependencies are
specified on sync objects, and dependence doesn't work between
kustomizations and helm releases, so to effect an ordering you have to
add an indirection through a sync. This is brittle because you end up
depending on a resource several hops away, _and_ it breaks reusability
because you have to mention the git repository itself.

HelmRelease is a cause of this problem too, as an indirection itself,
but might have to live with it.

**Treat secrets specially**

Secrets are not like other configuration. They should be supplied to
the runtime objects by the environment and provisioned out of
band. Design to accommodate platform services like KMS, and
third-party kit like Vault. Assume secrets have a different workflow
to other configuration, even if committed to the git repo.

**Encode the tenancy model**

Vertical tenancy (each tenant gets a set of clusters) and horizontal
tenancy (each tenant gets space on a set of clusters).

Including the platform v tenant distinction: some things are
cross-cutting and go everywhere, and some things are allocated
according to rules about tenancy.

# Non-goals, for the present

**No inversion of control between workers and control plane**

As in, the upstream plane cluster connects to the downstream cluster
to actuate things, and not the other way around.

**No hierarchy or distributed model**

This may be important at some stage; but using two tiers, control
plane -> workers, will get us quite far.

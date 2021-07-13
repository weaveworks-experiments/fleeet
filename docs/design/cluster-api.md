<!-- -8- fill-column: 100 -*- -->
# Bootstrapping Cluster API

**Status: thoughts**

What about making clusters at all?

That's a whole 'nother thing! But a really interesting one. So far I've looked only at how to
configure clusters when they arrive, in a GitOps way. But the other problem to solve is how to
create clusters in a GitOps way. As with configuration, there are two bits:

 - how do you bootstrap into a self-sustaining system
 - how do you continue from there, i.e., create and delete clusters.

The second requires further abstractions for groups of clusters, that I think will eventuate in
Cluster API. At which point they will just be usable by putting a definition in git. So for that
bit, I'm happy to just rely on tooling that will make filling out a cluster template easier.

The first part could be very useful -- it's essentially, how do I go from nothing, to a self-syncing
management cluster with the provider of my choice. This is like Flux bootstrapping but with two
extra bits:

 - installing the cluster API provider(s)
 - doing the dance of migrating from a temporary cluster to the real management cluster.

Installing providers is just like installing anything else, you just have to do it at the right
time. Specialised tooling could help.

The migration dance is something where tooling would really help, because it needs careful
co-ordination. Something like:

 - create a git repo with the provider definitions and flux config
 - commit a management cluster definition to the repo
 - create a temporary cluster, apply the provider and cluster definitions into it
 - when the new cluster is ready, pivot to it
 - run flux in the new cluster
 - throw away the temporary cluster

.. but it needs a lot of thinking through, since lots of things can fail, and there's lots of choices
to be made about the ordering; e.g.,

 - is it worth running flux on the temporary cluster
 - do you need to pivot in the cluster API sense if you are going to run flux on the new cluster
 - how do secrets work, as always

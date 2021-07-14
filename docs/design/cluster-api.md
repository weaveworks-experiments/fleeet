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

## Alternative sequence

The bootstrap cluster is throw-away; it's just to provision the real control cluster, once. So it
doesn't need Flux there. But we do want to end up with the definitions for the provider(s) in git;
and, there is the use of git for recovering from a failure in the process.

NB you might want a different set of providers in the control cluster than in the bootstrap cluster.

 1. Create a repo (if necessary)
 2. Create a temporary cluster
 3. Put the provider definitions in the repo, and apply them to the cluster (*)
 4. Put the control cluster definition in the repo, and apply it
 5. Wait for the control cluster to be ready
 6. Move the control cluster definition to the control cluster (**)
 7. Bootstrap Flux in the control cluster, with the repo already established.

You now have a self-sustaining control cluster.

* This has a kind of transactionality -- you don't want duff configuration in the repo.

** You have to let it be created, then move it, _then_ you can let Flux apply the definition. This
is because the definition is inherently runtime -- the provider fills in information that is not
present in the static definition. For it to represent the real cluster, it has to be created during
bootstrap then moved; applying the static definition would create it again.

## Open questions

**How do cluster moves work when you are going via GitOps?**

They can't work by just applying the definition in a new control cluster; you have to move the
definition, then you can let it be applied, just as in the note (**) above.

**How often do people need tooling for bootstrapping?**

Bootstrapping the whole thing is kind of a one-off -- is there something you would do more often?
Perhaps --

 - run a new control cluster and migrate to it
 - move "independent" clusters under control of a control cluster
 - do the above bootstrapping but with terraform

<!-- -*- fill-column: 100 -*- -->
# Configuration specialisation

**Status: drafting**

Every piece of configuration that is assigned to a cluster is likely
to need to be specialised to that cluster. For example, it might need
an IP address that depends on the data centre in which it runs, or to
refer to an image registry in the same availability zone, or to name
certain external resources after the cluster itself.

## Uses

**Interpolate a property of the cluster into configuration**

As an example, you have a configuration that creates some external
resources which are particular to the cluster, and to keep the
resources unique, you want to use the cluster name as a prefix. The
cluster name is available in the control plane, and you can control
the configuration so you can put substitution sites in.

**Adapt someone else's configuration**

Here, you want to use configuration owned by someone else. Rather than
vendor it, you would like to refer to it directly, but want to change
a few field values. The adaptation should be applied to all clusters.

**Interpolate a field from within the cluster into the configuration**

For example, each of your clusters is assigned an IP address which is
in a resource's field, within the cluster. You want the configuration
to have the IP address interpolated into it before applying.

**Interpolate a value from another module into a configuration**

In this case, the values to be interpolated are part of another module
(or bootstrap module). This is similar to interpolating a field from
within the cluster; but, the particular resource may need to be
resolved via the module.

## Questions

**Where does the specialisation run?**

Actually there are two questions here: where do the values for
specialisation get resolved, and where does the interpolation happen?

The design of assemblages and modules rules out specialising _only_ in
the control plane, since the idea is that mostly, the workload cluster
obtains its sources independently of the control plane. But that does
not rule out resolving values in the control plane, since those could
be passed along in the assemblage layer.

**How does interpolation work?**

Essentially, specialisation will mean taking some kind of template,
and interpolating values into it. There is one easily accessible,
reasonably general opportunity to modify the source before it is
applied, and that is by using the `postBuild` field of a
kustomization, which can substitute environment entries into fields
marked in the source configuration.

Other opportunities are:

 - use a composite source to mount a kustomization.yaml into the
   source used; especially useful when you don't own the configuration
   and can't put envsubst markers in
 - write a controller that can do more sophisticated specialisation,
   and use that instead of source-controller

Kustomize can help if the configuration cannot have substitution sites
put in it, e.g., if it belongs to someone else. However, you would
need to subsitute values into the kustomization itself, if they are
not literal, since Kustomize has no concept of parameters.

The above assumes it's the configuration that is specialised, and not
the sync. As an example of specialising the sync, you might want some
particular clusters to replace images. However: to the extent that
things in the sync are exposed in modules etc., this use is covered by
having different modules (and to the extent that things are not
exposed, you can't do it anyway).

**Where can values come from?**

There's a couple of reasons a cluster can be given specialisation
values:

 a. the values are particular to the cluster, and the configuration has
   slots defined for them;
 b. the configuration is generic and needs to be specialised for its
   use as a module.

In general, for a.) each cluster could have its own values; but there
will be uses for which it's handy to assign values in bulk, e.g., by
availability zone.

Values might come from different places, too:

 - as entries in a ConfigMap in the control plane
 - as field values in the Cluster object
 - as field values in an object associated with the Cluster object, e.g., a Machine
 - given in place in a spec

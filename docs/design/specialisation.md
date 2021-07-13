<!-- -*- fill-column: 100 -*- -->
# Configuration specialisation

**Status: drafting**

Every piece of configuration that is assigned to a cluster is likely to need to be specialised to
that cluster. For example, it might need an IP address that depends on the data centre in which it
runs, or to refer to an image registry in the same availability zone, or to name certain external
resources after the cluster itself.

## Uses

**Interpolate a property of the cluster into configuration**

As an example, you have a configuration that creates some external resources which are particular to
the cluster, and to keep the resources unique, you want to use the cluster name as a prefix. The
cluster name is available in the control plane, and you can control the configuration so you can put
substitution sites in.

**Adapt someone else's configuration**

Here, you want to use configuration owned by someone else. Rather than vendor it, you would like to
refer to it directly, but want to change a few field values. The adaptation should be applied to all
clusters.

**Interpolate a field from within the cluster into the configuration**

For example, each of your clusters is assigned an IP address which is in a resource's field, within
the cluster. You want the configuration to have the IP address interpolated into it before applying.

**Interpolate a value from another module into a configuration**

In this case, the values to be interpolated are part of another module (or bootstrap module). This
is similar to interpolating a field from within the cluster; but, the particular resource may need
to be resolved via the module.

## Discussion

**Where does the specialisation run?**

Actually there are two questions here: where do the values for specialisation get resolved, and
where does the interpolation happen?

The design of assemblages and modules rules out specialising _only_ in the control plane, since the
idea is that mostly, the workload cluster obtains its sources independently of the control
plane. But that does not rule out resolving values in the control plane, since those could be passed
along in the assemblage layer.

**How does interpolation work?**

Essentially, specialisation will mean taking some kind of template, and interpolating values into
it. There is one easily accessible, reasonably general opportunity to modify the source before it is
applied, and that is by using the `postBuild` field of a kustomization, which can substitute
environment entries into fields marked in the source configuration.

Other opportunities exist:

 - use a composite source to mount a kustomization.yaml into the source used; especially useful when
   you don't own the configuration and can't put envsubst markers in

Kustomize can help if the configuration cannot have substitution sites put in it, e.g., if it
belongs to someone else. However, you would need to subsitute values into the kustomization itself,
if they are not literal, since Kustomize has no concept of parameters.

You can create your own version of the first option above by making a git repository with a
kustomization in it, then substituting into the result with the envsubst mechanism. The question is
to what extent the system makes this easy for you, e.g., by taking a high-level description and
making the git repository available, composing it with other sources, etc. One small step would be
to represent composition in the API, so people don't have to vendor kustomizations in order to use
them in modules.

 - write a controller that can do more sophisticated specialisation, and use that instead of
   source-controller

Alternatively, bake the specialisation into source-controller so it's available everywhere.

All the above assumes it's the configuration that is specialised, and not the sync. As an example of
specialising the sync, you might want some particular clusters to replace images. However: to the
extent that things in the sync are exposed in modules etc., this use is covered by having different
modules (and to the extent that things are not exposed, you can't do it anyway).

**Where can values come from?**

There's a couple of reasons a cluster can be given specialisation values:

 - a. the values are particular to the cluster, and the configuration has slots defined for them;
 - b. the configuration is generic and needs to be specialised for its use as a module.

In general, for a.) each cluster could have its own values; but there will be uses for which it's
handy to assign values in bulk, e.g., by availability zone.

Values might come from different places, too:

 - as entries in a ConfigMap in the control plane
 - as field values in the Cluster object
 - as field values in an object associated with the Cluster object, e.g., a Machine
 - as a literal given in place in a spec

Note that none of these are "in the git repository"; the reason is that this is a runtime mechanism,
for doing what you can't already do with e.g., kustomize in the git repository.

## Design

In short:

 - you use directives in the module or bootstrap module spec to set environment variables for
   envsubst; these can be
   - a literal value
   - a data reference for getting a value from a ConfigMap or Secret
   - a field reference for getting a value from the Cluster object representing the workload cluster
   - a field reference for getting a value from an object in the workload cluster
   - (future: a reference to something in a module)
 - the directives are transmitted in the assemblage, and expanded to a `postBuild` section in the
   Kustomization object

### API

#### Bindings and variable mentions

**Syntax for binding and use of names**

Associating a value, or something that will be resolved to a value, to a name --- binding -- is
separate to using that name. This is to make it easier for names to be used in different
contexts. For example, instead of:

```yaml
spec:
  syncs:
  - kustomize:
      substitutions:
      - name: APP_NAME
        value: foo
```

the name is bound in a separate step, then mentioned:

```yaml
spec:
  environment:
  - name: APP_NAME
    value: foo
  syncs:
  - kustomize:
      substitutions:
      - APP_NAME: $(APP_NAME)
```

That way,

 - environment entries could be used in e.g., a Helm values object, and work the same way
 - environment entries can be bound once then used in more than one context; e.g., in more than one
   sync

The downside to this is that it can feel like you are repeating yourself, if the destination is also
something like an environment as above (`APP_NAME: $(APP_NAME)`). Special syntax for mentioning a
binding may be in order for those situations:

```yaml
spec:
  syncs:
  - kustomize:
      substitutions:
      - binding: APP_NAME
      - name: DEFINED_HERE
        value: foo
```

A potential weakness is that the syntax for referring to an environment entry will need to be chosen
so it doesn't collide with other substitution mechanisms. If you have a module definition that is
synced by Flux (which is likely, since that's the idea) and envsubst is invoked, it will replace
`${APP_NAME}` with (probably) an empty string, before it's seen by Fleeet. Kubernetes uses `$(...)`,
while envsubst as used by Flux uses `${...}`. Provided you can escape a literal `$()`, that will
work.

**Resolution of binding values**

As above, there are these kinds of binding:

 - a literal value
 - a specific data value from a ConfigMap or Secret
 - a reference to a field in an arbitrary resource
 - a reference to a field in the objects defining the cluster
 - a reference to a field in a resource in the workload cluster itself

Literal values don't serve a purpose for the user other than reducing the number of places they have
to repeat a value. However, they may be useful for carrying already-evaluated values to the
downstream, and for "showing working":

```yaml
spec:
  bindings:
  - name: APP_NAME
    value: frob
  - name: APP_NAMESPACE
    value: $(APP_NAME)-ns
```

Values in `ConfigMap` and `Secret` objects are just fields under `.data`, so there is no need to
have a special case for those.

```yaml
spec:
  bindings:
  - name: APP_NAME
    objectFieldReference:
      kind: ConfigMap
      name: app_config
      path: .data.AppName
  - name: API_HOST
    objectFieldReference:
      kind: Cluster
      name: $(CLUSTER_NAME)
      path: .spec.controlPlaneEndpoint.host
```

If you have the ability to name an object, and you have the name of the target cluster available,
you can refer to an arbitrary object or to the cluster object (and possibly to another object
related to the cluster object, if you've been careful in your naming). So one design is to make the
target cluster name available _a priori_, and let people interpolate into the names of object field
references:

```yaml
spec:
  bindings:
  - name: APP_NAME
    objectFieldReference:
      kind: ConfigMap
      name: app_config
      path: .data.AppName
  - name: API_HOST
    objectFieldReference:
      kind: Cluster
      name: $(CLUSTER_NAME)
      path: .spec.controlPlaneEndpoint.host
```

This has the added complexity of having a magic binding, and of needing to interpolate into other
binding definitions. But it is very flexible.

Alternatively, the cluster definition can be distinguished syntactically:

```yaml
spec:
  bindings:
  - name: APP_NAME
    objectFieldReference:
      kind: ConfigMap
      name: app_config
      path: .data.AppName
  - name: API_HOST
    clusterFieldReference:
      path: .spec.controlPlaneEndpoint.host
```

This suffers from being extra syntax, and being very narrow -- how do you refer to some other object
related to the cluster, for example.

**Upstream vs downstream**

It needs to be specified whether an object to be resolved is in the source cluster or the
destination cluster; and there is a benefit to this being clear to the user writing or reading a
spec, as well. Since different types will have one or other or both (e.g., an Assemblage won't have
upstream bindings), there's a strong indication that bindings should be segregated into separate
fields entirely.

Indeed, it's easier to implement and to understand as a user if the source cluster bindings are
obviously evaluated before the target cluster bindings. (I'm sure it is possible technically to find
a fixed point when anything can refer to anything, but there is less magic if it can't.)

For those reasons, there should be a section for bindings involving upstream objects, and a section
for bindings involving downstream objects:

```yaml
spec:
  upstreamBindings:
  ...
  downstreamBindings:
  ...
```

or

```yaml
spec:
  bindings:
    upstream:
    ...
    downstream:
```

For consistency, it's better if the same field names are used and are just present or
absent. Therefore, maybe:

```yaml
spec:
   bindings: # evaluated here
   ...
   downstreamBindings: # evaluated downstream
   ...
```

**Order of evaluation**

One of the examples above assumes that bindings are available when evaluating bindings
themselves. This choice makes the implementation and comprehension a little more complicated, but
opens up possibilities for the user.

When do mentions of names outside the bindings get evaluated? Since it's possible to still have
unreolved bindings in the upstream cluster, this has to happen in the downstream cluster. This means
you can't have anything that needs to be used in the upstream cluster be a site for interpolation;
e.g., the selector of a Module cannot use substitutions, and neither can the name of a sync.

Therefore it seems better to selectively allow substitutions; e.g., in certain fields of each sync
description. Since a module is a _specific_ config at a _specific_ version, the git repository and
version are probably out, at least to start with. This leaves the `package` field, or specific
fields therein.

**Interpolation of values**

Since a field value can have an aggregate type (e.g., a list), but variable mentions can be in the
middle of a value (e.g., `"--app $(app_name)"`) , values will need to be stringified. There's a
choice of how to stringify values.

TODO

## How do you do ...?

**Interpolate a property of the cluster into configuration**

 - Add an envsubst marker to your configuration (or create a kustomization which patches the
   envsubst marker in);
 - In your Module spec, use a directive to refer to a Cluster field. When the module is assigned to
   a cluster, the value will be filled in.

**Adapt someone else's configuration**

Let's assume the original configuration does not have any envsubst markers in it. You will need to
figure out where you want to interpolate values into, then make a kustomization that patches those
values into the right places.

In GitOps Toolkit you might compose sources together to avoid using submodules or vendoring the
original configuration; in the absence of a mechanism for giving composite sources here, I'm going
to just use submodules. Aside from not requiring further design, an advantage is that the whole
configuration is available in the git repository, rather than being assembled at runtime, so you can
verify it statically.

Therefore:

 - add the original configuration as a submodule to your git repository;
 - create a kusotmization that uses the submodule directory as base, and patches it with envsubst
   markers;
 - give literal values for the envsubst markers, in the Module spec.

**Interpolate a field from within the cluster into the configuration**

 - Put envsubst markers into your configuration, or create a kustomization which patches them in;
 - In your Module spec, use directives to get values for envsubst from the desired workload cluster
   resource.

**Interpolate a value from another module into a configuration**

This is left for a future design.

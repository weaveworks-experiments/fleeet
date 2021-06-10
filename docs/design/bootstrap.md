<!-- -*- mode: markdown; fill-column: 100 -*- -->
# Bootstrapping

**Status: under development**

There are two layers on which you need bootstrapping:

 1. a Fleeet system on an upstream (management) cluster
 2. the required components on each downstream cluster

## Bootstrapping an upstream cluster

The desired end point is that there's a self-sustaining sync for the upstream cluster. The
configuration defines its own machinery -- deployments for the source-controller and
kustomize-controller from GitOps Toolkit, with a deployment for the remote assemblage and module
controllers from here; plus, a `GitRepository` and `Kustomization` for syncing the configuration
itself.

The `flux` command line tool does a lot of the work of setting up Git repositories with a
self-sustaining configuration. What is needed beyond that is to commit the definitions and
deployments from here to the Git repository.

## Bootstrapping downstream clusters

The `BootstrapModule` type is designed for the latter kind of bootstrap. You specify what needs to
be present on every downstream cluster, and it will be applied directly. To bootstrap, this needs to
be the source-controller and kustomize-controller from [GitOps Toolkit](https://toolkit.fluxcd.io/),
and the assemblage-controller from here.

In a typical GitOps Toolkit installation, each cluster has its own Git repository (or part of a Git
repository -- a directory or branch). However, a central premise of fleet GitOps is to _avoid_
materialising a configuration for every cluster, so this machinery does not bootstrap each
downstream cluster so as to sustain its own machinery. Instead, the machinery is defined centrally
with the `BootstrapModule`.

## Bootstrapping a fleet

Here's how you would do it, minimally, by hand:

 - Run `flux bootstrap --components=kustomize-controller,source-controller`, targeting the
   management cluster;
 - Add manifests for the `RemoteAssemblage`, `Module` and `BootstrapModule` custom resource
   definitions, and a deployment to run the module controller, to the management cluster git
   repository;
 - Create a configuration for running kustomize-controller, source-controller, and
   assemblage-controller, and put that in a Git repository;
 - Add a `BootstrapModule` that refers to the configuration from above, and assign it to all
   downstream clusters.

You are now ready to add modules to the management cluster, and they will be deployed to any
downstream clusters as assigned.

## Open questions

**To what extent does it make sense to automate the steps given above?**

**Is it worth having a CLI tool for helping?**

Provisional answer: I think the ideal is that other tools (`flux`, `git`) are sufficient, and this
project includes only the configurations used with those. The steps not covered by this will be
conveniently creating the fleeet-specific objects like `BootstrapModule`.

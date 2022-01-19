<!-- -*- fill-column: 100 -*- -->
# Installation instructions

This is a step-by-step recipe for installing Fleeet.

## Prerequisites

You will need:

 - a Kubernetes cluster to use as a control-plane cluster
 - a GitHub account (or equivalent, if you are prepared to adapt the example commands given below)
 - the [`flux` command-line tool][flux download]

## Install Flux in the control-plane cluster

To go directly to driving changes through Git, start by bootstrapping Flux in the control-plane
cluster. You will need to provide your own values for `OWNER` and `REPO` in the following.

```bash
OWNER=squaremo
REPO=control-plane-config
flux bootstrap --components=kustomize-controller,source-controller github \
               --owner $OWNER --repository $REPO --personal --path=./upstream
```

The command above will create the named GitHub project if it does not exist, commit a configuration
that runs the kustomize- and source-controller parts of Flux, and commit Flux API objects that will
sync the cluster to the same repository. Aside from limiting the controllers, this is a standard
Flux bootstrap.

## Install the control-plane component via the Git repository

After bootstrapping Flux, to affect the cluster you will make changes in Git.

Clone the repository used by Flux:

```bash
git clone ssh://git@github.com/$OWNER/$REPO fleet-config
cd fleet-config
```

```bash
CONFIG_REPO=https://github.com/squaremo/fleeet-config
flux create source git --branch main --url "$CONFIG_REPO" fleeet-config --export > upstream/fleeet-config-source.yaml
flux create kustomization cluster-api --source=fleeet-config --path=configs/cluster-api --prune=true --export > upstream/cluster-api-sync.yaml
flux create kustomization fleeet-control --source=fleeet-config --path=configs/fleeet-control/default --prune=true --depends-on=cluster-api --export > upstream/fleeet-sync.yaml
```



[flux download]: 

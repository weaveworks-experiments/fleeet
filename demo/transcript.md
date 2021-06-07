<!-- fill-column: 100 -->
# Transcript of creating a Fleeet system

## 1. Bootstrap syncing on the management cluster

This will create a self-sustaining sync mechanism on the management cluster. This means I can add
more things to git and they will be synced.

With a default context of the intended management cluster:

```bash
OWNER=squaremo
REPO=fleeet-demo
flux bootstrap --components=kustomize-controller,source-controller github --private=false --owner $OWNER --repository $REPO --personal --path=./upstream
```

This installs the required Flux custom resources and controllers as a configuration in a fresh git
repository, including a sync referring to the git repository so as to sustain that configuration in
the cluster; then, applies the configuration to the cluster to bootstrap it.

## 2. Install module layer components to the management cluster

This adds configuration for the Fleeet control layer to the management git repository, so they will
be created in the management cluster via the sync machinery bootstrapped in the previous step.

Get the management cluster git repository:

```bash
git clone "git@github.com:$OWNER/$REPO" demo
pushd demo
```

Get the configs needed, from upstream repositories:

```bash
mkdir -p configs
# In lieu of installing Cluster API, which I don't need, just use the CRDs
kpt pkg get https://github.com/kubernetes-sigs/cluster-api.git/config/crd/bases@master configs/cluster-api
#
# The following gets the control layer config subdirectory into this repo,
# keeping track of its origin so it can updated later. Just fetching the files,
# or using a kustomization with a remote base, could be alternatives.
kpt pkg get ssh://git@github.com/squaremo/fleeet.git/control/config@bootstrap-demo configs/fleeet-control
```

Create syncs that will install the CRDs and controllers:

```bash
# Sync the Cluster API definitions
flux create kustomization cluster-api --source=flux-system --path=configs/cluster-api --prune=true --export > upstream/cluster-api-sync.yaml
#
# Sync the fleeet control layer definitions
flux create kustomization fleeet-control --source=flux-system --path=configs/fleeet-control/default --prune=true --depends-on=cluster-api --export > upstream/fleeet-sync.yaml
#
# Add all of these to git
git add configs/cluster-api configs/fleeet-system upstream/{fleeet,cluster-api}-sync.yaml
git commit -s -m "Add Cluster API and fleeet component sync"
git push
```

Now the sync machinery will create extra syncs for the Cluster API definitions and the fleeet
control layer definitions. The end result should be that a working control layer controller will be
started:

```bash
flux reconcile kustomization --with-source flux-system
kubectl -n control-system logs deploy/control-controller-manager -f
```

## 3. Create a bootstrap module that will install the assemblage-level controller

This ties another knot: how does a downstream cluster start syncing anything? The answer is to make
a `BootstrapModule`, which will install the required GitOps Toolkit and Fleeet machinery on each
downstream cluster.

First, create a configuration that will install GitOps Toolkit components:

```bash
# Create a configuration for GitOps Toolkit components
mkdir -p configs/flux-worker
flux install --export --components=kustomize-controller,source-controller > configs/flux-worker/flux-components.yaml
#
# Get an assemblage layer (downstream) configuration to sync to downstream clusters
kpt pkg get ssh://git@github.com/squaremo/fleeet.git/assemblage/config@bootstrap-demo configs/fleeet-worker
# Add these to the git repository
git add configs/{flux-worker,fleeet-worker}
git commit -s -m "Add downstream bootstrap configs"
```

Now create bootstrap modules referring to these bits of repository:

```bash
# This bootstrap module will be applied to all downstream clusters that show up in the namespace. The module must be given a particular revision or tag (but not a branch -- that would be the same as using image:latest).
REV=$(git rev-parse HEAD) cat > upstream/bootstrap-worker.yaml <<EOF
---
apiVersion: fleet.squaremo.dev/v1alpha1
kind: BootstrapModule
metadata:
  name: flux-components
spec:
    selector:
      matchLabels: {}
    sync:
      source:
        git:
          url: https://github.com/$OWNER/$REPO
          version:
            revision: "$REV"
      package:
        kustomize:
          path: ./configs/flux-worker
---
apiVersion: fleet.squaremo.dev/v1alpha1
kind: BootstrapModule
metadata:
  name: assemblage-controller
spec:
    selector:
      matchLabels: {}
    sync:
      source:
        git:
          url: https://github.com/$OWNER/$REPO
          version:
            revision: "$REV"
      package:
        kustomize:
          path: ./configs/fleeet-worker/default
EOF
#
# Add it to git, to be synced to the management cluster
git add upstream/bootstrap-worker.yaml
git commit -s -m "Add bootstrap module for downstream"
```

TODO: a diagram of the situation at this point.

## 4. Make a cluster and see what happens

```bash
# back to the demo directory
popd
#
# Use the supplied script to create a cluster
sh create-cluster.sh cluster-1
```

### Create a module

```bash
# Create a module and add that to syncing
cat > upstream/podinfo-module.yaml <<EOF
apiVersion: fleet.squaremo.dev/v1alpha1
kind: Module
metadata:
  name: podinfo
  namespace: default
spec:
  selector: {}
  sync:
    source:
      git:
        url: https://github.com/stefanprodan/podinfo
        version:
          tag: 5.2.0
    package:
      kustomize:
        path: ./kustomize
EOF
git add upstream/podinfo-module.yaml
git commit -s -m "Add module for podinfo app"
git push
```

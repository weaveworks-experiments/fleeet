<!-- fill-column: 100 -->
# Transcript of creating a Fleeet system

<!-- TODO: diagrams of the syncs and clusters at each point -->

## _0. Prerequisites_

You need a

 - control cluster that's easy to repurpose and recycle (I use Docker Desktop)
 - a Docker to run kind clusters (it could be the same Docker as above; I use a desktop PC)

You need the fleeet repo cloned on the computer that will run the `kind` clusters, so you have
access to the scripts in it.

### _0.1. Adapt kind.config to the local environment_

On the computer running `kind`, edit `demo/kind.config` so that the `apiServerAddress` field has the
IP assigned to en0 (or eth0, or whichever is your "main" interface). This is so that the clusters
created with `kind` will be accessible from the control cluster; otherwise, they will listen on
localhost and the control cluster won't be able to control them.

### _0.2. Let the kind computer connect to the control cluster_

If these are different computers, you'll need the control cluster available from the kind
commputer. I copied the kubeconfig over, and I use an SSH tunnel which is established when I log in
to the kind computer:

    ssh -R 6443:kubernetes.docker.internal:6443 mikeb@kind.local

(`kubernetes.docker.internal` is the alias given in the kubeconfig, that will resolve to the Docker
Desktop. It may differ in your setup.)

### _0.3. Vendor useful config into a repo_

Do this once, or use my fleeet-config repository.

```bash
mkdir -p fleeet-config/configs
cd fleeet-config
git init
hub create # or create a repo in github, and set it as the origin
#
# In lieu of installing Cluster API, which I don't need, just use the CRDs
kpt pkg get https://github.com/kubernetes-sigs/cluster-api.git/config/crd/bases@master configs/cluster-api
#
# The following gets the control layer config subdirectory into this repo,
# keeping track of its origin so it can updated later. Just fetching the files,
# or using a kustomization with a remote base, could be alternatives.
kpt pkg get ssh://git@github.com/squaremo/fleeet.git/control/config@main configs/fleeet-control
#
# Create a configuration for GitOps Toolkit components
mkdir configs/flux-worker
flux install --export --components=kustomize-controller,source-controller > configs/flux-worker/flux-components.yaml
#
# Get an assemblage layer (downstream) configuration to sync to downstream clusters
kpt pkg get ssh://git@github.com/squaremo/fleeet.git/assemblage/config@main configs/fleeet-worker
# Add these to the git repository
git add configs
git commit -s -m "Add vendored configs"
#
# Tag a version so it can be used in the version field
git tag -a v0.1
git push origin main v0.1
```

I also tag a base, so I can reset to it if I pile commits on for demo purposes:

```bash
git tag -a base
git push origin base
```

## 1. Bootstrap syncing on the management cluster

This will create a self-sustaining sync mechanism on the management cluster. This means I can do
everything through git, and it will be synced.

Assuming a default context of the intended management cluster:

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

Get the control cluster git repository that was made (or populated) by Flux while bootstrapping:

```bash
git clone "git@github.com:$OWNER/$REPO" demo
cd demo
```

Create syncs that will install the CRDs and controllers:

```bash
# The repo into which you vendored all the configs; you can use this one, it's public and you only need read access.
CONFIG_REPO=https://github.com/squaremo/fleeet-config
#
# Create a GitRepository for the config repo
flux create source git --branch main --url "$CONFIG_REPO" fleeet-config --export > upstream/fleeet-config-source.yaml
#
# Sync the Cluster API definitions
flux create kustomization cluster-api --source=fleeet-config --path=configs/cluster-api --prune=true --export > upstream/cluster-api-sync.yaml
#
# Sync the fleeet control layer definitions
flux create kustomization fleeet-control --source=fleeet-config --path=configs/fleeet-control/default --prune=true --depends-on=cluster-api --export > upstream/fleeet-sync.yaml
#
# Add all of these to git
git add upstream/{fleeet,cluster-api}-sync.yaml upstream/fleeet-config-source.yaml
git commit -s -m "Add config source and fleeet component syncs"
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

Now create bootstrap modules referring to these bits of repository:

```bash
# This bootstrap module will be applied to all downstream clusters that show up in the namespace. The module must be given a particular revision or tag (but not a branch -- that would be the same as using image:latest).
CONFIG_VERSION=v0.1
cat > upstream/bootstrap-worker.yaml <<EOF
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
          url: $CONFIG_REPO
          version:
            tag: "$CONFIG_VERSION"
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
          url: $CONFIG_REPO
          version:
            tag: "$CONFIG_VERSION"
      package:
        kustomize:
          path: ./configs/fleeet-worker/default
EOF
#
# Add it to git, to be synced to the management cluster
git add upstream/bootstrap-worker.yaml
git commit -s -m "Add bootstrap modules for downstream"
git push
```

## 4. Make a cluster and see what happens

```bash
# In the fleeet/demo directory on the kind computer:
#
# Use the supplied script to create a cluster
sh create-cluster.sh cluster-1
```

See what happened in the downstream cluster:

```bash
kubectl --kubeconfig ./cluster-1.kubeconfig get namespace
# and explore from there ...
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

When `flux-system` is synced, the module will be created, and a proxy set up for each cluster to
sync the module:

```bash
# This shows the module ..
$ kubectl get module
NAME      REVISION          TOTAL   UPDATING   SUCCEEDED   FAILED
podinfo   {"tag":"5.2.0"}   2       2          0           0
#
# .. this shows the proxy assemblages
$ kubectl get remoteassemblage
NAME        AGE
cluster-a   4m14s
cluster-b   4m14s
#
# .. and this shows the effect in the downstream cluster
kubectl --kubeconfig ./cluster-a.kubeconfig get deploy
NAME      READY   UP-TO-DATE   AVAILABLE   AGE
podinfo   2/2     2            2           5m17s
```

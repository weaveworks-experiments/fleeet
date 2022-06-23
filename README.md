# Fleeet Demo Instructions

You'll need a least 2 different terminals / panes.

This demo uses [`k9s`][k9s], but you can use whichever tool you like to view clusters.

1. Create a PAT with `repo` scope and export it to `GITHUB_TOKEN`.
1. Edit `kind.yaml` and set the value of **apiServerAddress** to your machines ip address
  (usually the first one from `hostname -I`)
1. Edit `tilt-settings-tenant.json` and set the value of `repo_owner` to be your username or org.
1. Edit `tilt-settings-mgmt.json` and set the value of `repo_owner` to be your username or org.
1. (optional) If you are SSH-ed into another machine on your LAN, add `--host 0.0.0.0` to your
  `tilt` commands in `dev-start-tenant.sh` and `dev-start-mgmt.sh`.
1. Open a terminal window and run the following:
    ```bash
    ./dev-start-first.sh
    ./dev-start-tenant.sh 0
    ```
1. Open the Tilt browser window (by pressing space) and wait for everything to go green.
1. (optional) Open another terminal window and run
    ```bash
    k9s --kubeconfig .tiltbuild/tenant-fleeet-tenant-0.kubeconfig
    ```
1. Open another terminal window and run the following:
    ```bash
    ./dev-start-mgmt.sh
    ```
1. Open the Tilt browser window (by pressing space) and wait for everything to go green.
1. (optional) Open another terminal window and run:
    ```bash
    k9s --kubeconfig mgmt.kubeconfig
    ```
1. Clone down your `fleeet-demo` repo which has been created in your Github and change into it.
1. Create the following `Kustomization` in `mgmt/fleet-objects-sync.yaml`
    ```bash
    cat <<EOF > mgmt/fleet-objects-sync.yaml
    ---
    apiVersion: kustomize.toolkit.fluxcd.io/v1beta1
    kind: Kustomization
    metadata:
      name: fleet-objects
      namespace: flux-system
    spec:
      interval: 1m0s
      path: ./fleet
      prune: true
      sourceRef:
	kind: GitRepository
	name: flux-system
    EOF
    ```
1. Create a new `podinfo` module in a `fleet/` directory:
    ```bash
    cat <<EOF > fleet/podinfo-module.yaml
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
	    url: https://github.com/richardcase/podinfo
	    version:
	      tag: v0.1.0
	package:
	  kustomize:
	    path: ./kustomize
    EOF
    ```
1. Commit and push your changes to your `fleeet-demo` repo.

What you should see happen:
* In the management cluster you should see a module and remoteassemblage created
* In the tenant cluster you should see the following created:
  * assemblage
  * gitrepository
  * kustomization
  * Pods/svc for podinfo


[k9s]: https://k9scli.io/

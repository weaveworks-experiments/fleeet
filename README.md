# Fleeet Demo Instructions

You'll need a least 2 different terminals / panes.

1. Edit `kind.yaml` and set the value of **apiServerAddress** to your machines ip address
2. Open a terminal window and run the following:
```bash
./dev-start-first.sh
./dev-start-tenant.sh 0
```
3. Open the Tilt browser window (by pressing space) and wait for everything to go green.
4. (optional) Open another terminal window and run
```bash
k9s --kubeconfig .tiltbuild/tenant-fleeet-tenant-0.kubeconfig
```
5. Open another terminal window and run the following:
```bash
./dev-start-mgmt.sh
```
6. Open the Tilt browser window (by pressing space) and wait for everything to go green.
7. (optional) Open another terminal window and run:
```bash
k9s --kubeconfig mgmt.kubeconfig
```

What you should see happen:
* In the management cluster (using k9s from step 7) you should see a module and remoteassemblage created
* In the tenant cluster (using the k9s from step 4) you should see the following created:
  * assemblage
  * gitrepository
  * kustomization
  * Pods/svc for podinfo
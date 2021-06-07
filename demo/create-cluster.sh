set -e
set -o pipefail

images="squaremo/fleeet-assemblage:latest"

if [ -z "$1" ]; then
    echo "Usage: create-cluster.sh <name>" >&2
    exit 1
fi

name="$1"

echo "--> creating kind cluster $name"
kind create cluster --kubeconfig "./$name.kubeconfig" --name "$name" --config=./kind.config

echo "--> side-loading controller images"
for image in "$images"; do kind load docker-image --name "$name" "$image"; done

echo "--> create secret $name-kubeconfig"
kubectl create secret generic "$name-kubeconfig" --from-file="value=./$name.kubeconfig"

port=$(yq r "$name.kubeconfig" 'clusters[0].cluster.server' | sed 's#https://.*:\([0-9]\{4,5\}\)#\1#')

echo "--> writing and applying cluster manifest $name.yaml"
cat > "$name.yaml" <<EOF
apiVersion: cluster.x-k8s.io/v1alpha4
kind: Cluster
metadata:
  name: $name
  namespace: default
spec:
  controlPlaneEndpoint:
    host: 192.168.86.23
    port: $port
status:
  infrastructureReady: true
  controlPlaneReady: true
EOF

kubectl apply -f "$name.yaml"

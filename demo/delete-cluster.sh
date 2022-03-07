set -e
set -o pipefail

if [ -z "$1" ]; then
    echo "Usage: delete-cluster.sh <name>" >&2
    exit 1
fi

name="$1"

echo "--> removing Cluster object and secret"
kubectl delete --ignore-not-found secret "$name-kubeconfig"
if [ -f "$name.yaml" ]; then kubectl delete --ignore-not-found -f "$name.yaml"; fi

echo "--> deleting kind cluster $name"
kind delete cluster --name "$name"

echo "--> cleaning up kubeconfig and YAML"
rm -f "$name.kubeconfig"
rm -f "$name.yaml"

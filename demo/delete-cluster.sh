set -e
set -o pipefail

if [ -z "$1" ]; then
    echo "Usage: create-cluster.sh <name>" >&2
    exit 1
fi

name="$1"

echo "--> removing Cluster object and secret"
kubectl delete -f "$name.yaml"
kubectl delete secret "$name-kubeconfig"

echo "--> deleting kind cluster $name"
kind delete cluster --name "$name"

echo "--> cleaning up kubeconfig"
rm "$name.kubeconfig"

set -e
set -o pipefail

if [ -z "$1" ]; then
    echo "Usage: enrol-kind-cluster.sh <kubeconfig>" >&2
    exit 1
fi

if [ ! -f "$1" ]; then
    echo "Did not find file $1" >&2
fi

kubeconfig="$1"
name="${kubeconfig%.kubeconfig}"

echo "--> create secret $name-kubeconfig"
kubectl create secret generic "$name-kubeconfig" --from-file="value=$kubeconfig"

host=$(yq eval '.networking.apiServerAddress' "kind.config")
port=$(yq eval '.clusters[0].cluster.server'"$kubeconfig" | sed 's#https://.*:\([0-9]\{4,5\}\)#\1#')

echo "<!> Using host $host from kind.config apiServerAddress, this is assumed to be"
echo "    an IP address accessible from the control cluster. For example, the IP address"
echo "    assigned to en0 would usually work if the various hosts are on the same network."

echo "--> writing and applying cluster manifest $name.yaml"
cat > "$name.yaml" <<EOF
apiVersion: cluster.x-k8s.io/v1alpha4
kind: Cluster
metadata:
  name: $name
  namespace: default
spec:
  controlPlaneEndpoint:
    host: $host
    port: $port
status:
  infrastructureReady: true
  controlPlaneReady: true
EOF

kubectl apply -f "$name.yaml"

echo "DONE"

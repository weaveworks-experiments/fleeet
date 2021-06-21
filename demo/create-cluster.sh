set -e
set -o pipefail

images="squaremo/fleeet-assemblage:latest"

if [ -z "$1" ]; then
    echo "Usage: create-cluster.sh <name>" >&2
    exit 1
fi

name="$1"

echo "--> creating kind cluster $name with kubeconfig $name.kubeconfig"
kind create cluster --kubeconfig "./$name.kubeconfig" --name "$name" --config=./kind.config

echo "--> side-loading controller images"
for image in "$images"; do kind load docker-image --name "$name" "$image"; done

echo "DONE"

#!/usr/bin/env bash

set -e
set -o pipefail

TENANT_NUM=$1
KIND_PORT=$((10351 + TENANT_NUM))

TENANT_NAME="fleeet-tenant-$TENANT_NUM"

kind create cluster --name "$TENANT_NAME" --config kind.yaml

kind get kubeconfig --name "$TENANT_NAME" > ".tiltbuild/tenant-fleeet-tenant-$TENANT_NUM.kubeconfig"

tilt --file Tiltfile-tenant -v --port "$KIND_PORT" up

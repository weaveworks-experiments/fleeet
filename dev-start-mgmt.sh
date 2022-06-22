#!/usr/bin/env bash

set -e
set -o pipefail

kind create cluster --name fleeet-mgmt --config kind.yaml
kind get kubeconfig --name fleeet-mgmt > mgmt.kubeconfig

tilt --file Tiltfile-mgmt  -v up

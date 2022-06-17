#!/usr/bin/env bash

set -e
set -o pipefail

kind create cluster --name fleeet-mgmt

tilt --file Tiltfile-mgmt --host 0.0.0.0 -v up

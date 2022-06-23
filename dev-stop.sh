#!/usr/bin/env bash

set -e
set -o pipefail

kind get clusters | grep fleeet- | xargs -I% kind delete cluster --name=%

pkill -f tilt

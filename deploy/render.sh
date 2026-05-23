#!/usr/bin/env sh
set -e
: "${VERSION:?VERSION not set}"
sed "s/__VERSION__/${VERSION}/g" "$(dirname "$0")/kpulse.yaml.tmpl"

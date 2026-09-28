#!/bin/sh
# Delete the local kind cluster named "forgelab" (the whole lab cluster).
set -eu
kind delete cluster --name forgelab

#!/bin/bash

set -euo pipefail

mkdir -p ~/.ssh

echo "$GCP_SSH_PRIVATE_KEY" > ~/.ssh/id_ed25519
chmod 600 ~/.ssh/id_ed25519

ssh-keyscan -H "$GCP_VM_IP" >> ~/.ssh/known_hosts
#!/bin/bash
set -e

echo "Cloning isolate source"
git clone https://github.com/ioi/isolate.git

echo "Installing dependencies"
sudo apt install -y pkg-config libcap-dev libseccomp-dev libsystemd-dev

echo "Building"
cd isolate
make isolate
sudo make install
sudo chmod u+s /usr/local/bin/isolate
cd ..

echo "Setting up subuid/subgid"
grep -q "^isolate:" /etc/subuid || echo "isolate:100000:65536" | sudo tee -a /etc/subuid
grep -q "^isolate:" /etc/subgid || echo "isolate:100000:65536" | sudo tee -a /etc/subgid

echo "Creating isolate system user if not present"
id isolate &>/dev/null || sudo useradd --system --no-create-home isolate

echo "Initializing isolate for user: $USER"
sudo isolate --as-uid=$(id -u $USER) --as-gid=$(id -g $USER) --init


if [-e ./isolate ]; then 
    rm -rf ./isolate
fi
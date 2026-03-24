#! /bin/bash

# should install the system from GitHub
gh repo clone krbundy/psaec

cd psaec


npm ci 
chmod +x ./build.sh
./build.sh

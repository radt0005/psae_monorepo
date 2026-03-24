#! /bin/bash

echo "Starting build process..."



if [ -e "./.target" ]; then
    echo "Removing existing build directory"
    rm -rf ./.target
fi

mkdir "./.target"
mkdir "./.target/hooks/"
mkdir "./.target/hooks/collections"
mkdir "./.target/pb_public/"


echo "Building Client Application\n\n"
npm run build


echo "Compiling PocketBase hooks"
tsc  ./hooks/runs.ts --outDir ./hooks/ --target es6 --module commonjs 

echo "Copying files"
cp -r ./.output ./.target/pb_public/
cp -r ./hooks/runs.js ./.target/hooks/collections/runs.js
cp ./setup.sh ./.target/setup.sh

echo "Creating zip archive"
tar -czvf bundle.tar.gz ./.target


echo "Done"


# bundle the code as a zip

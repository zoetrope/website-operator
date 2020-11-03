#!/bin/bash -ex
cd $HOME
rm -rf $REPO_NAME
git clone $REPO_URL
cd $REPO_NAME
git checkout $REVISION

npm install
npm run build

rm -rf $OUTPUT/*
cp -r public/* $OUTPUT/

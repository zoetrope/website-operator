#!/bin/bash -ex
cd $HOME
rm -rf $REPO_NAME
git clone $REPO_URL
cd $REPO_NAME
git checkout $REVISION

pip3 install -r requirements.txt
export PATH=$PATH:$HOME/.local/bin
mkdocs build

rm -rf $OUTPUT/*
cp -r site/* $OUTPUT/

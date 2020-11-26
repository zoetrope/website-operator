#!/bin/bash -ex
cd $HOME
rm -rf $REPO_NAME
git clone $REPO_URL
cd $REPO_NAME
git checkout $REVISION

sed -i -e "/host/c\      \"host\": \"http://${RESOURCE_NAME}.${RESOURCE_NAMESPACE}.example.com/es\"," book.js
sed -i -e "/index/c\      \"index\": \"${RESOURCE_NAME}-${REVISION}\"," book.js

npm install
npm run build

curl -X DELETE ${ELASTIC_HOST}/${RESOURCE_NAME}-${REVISION}
curl -X PUT ${ELASTIC_HOST}/${RESOURCE_NAME}-${REVISION} -H 'Content-Type: application/json' -d @mappings.json
curl -X POST ${ELASTIC_HOST}/${RESOURCE_NAME}-${REVISION}/_bulk -H 'Content-Type: application/json' --data-binary @_book/search_index.json

rm -rf $OUTPUT/*
cp -r _book/* $OUTPUT/
cp -r assets/* $OUTPUT/assets/

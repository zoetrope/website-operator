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

curl -X DELETE http://honkit-es:9200/${REPO_NAME}-${REVISION}
curl -X PUT http://honkit-es:9200/${REPO_NAME}-${REVISION} -H 'Content-Type: application/json' -d @mappings.json
curl -X POST http://honkit-es:9200/${REPO_NAME}-${REVISION}/_bulk -H 'Content-Type: application/json' --data-binary @_book/search_index.json

rm -rf /data/*
cp -r _book/* /data/
cp -r assets/* /data/assets/

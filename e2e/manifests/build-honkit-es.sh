#!/bin/bash -ex
cd $HOME

rm -rf $REPO_NAME
git clone $REPO_URL

cd $REPO_NAME
git checkout $REVISION

sed -i -e "/host/c\      \"host\": \"http://website-operator.example.com/es\"," book.js
sed -i -e "/index/c\      \"index\": \"${REPO_NAME}-${REVISION}\"," book.js

npm install
npm run build

curl -X DELETE http://${REPO_NAME}:9200/${REPO_NAME}-${REVISION}
curl -X PUT http://${REPO_NAME}:9200/${REPO_NAME}-${REVISION} -H 'Content-Type: application/json' -d @mappings.json
curl -X POST http://${REPO_NAME}:9200/${REPO_NAME}-${REVISION}/_bulk -H 'Content-Type: application/json' --data-binary @_book/search_index.json

rm -rf /data/
cp -r _book/* /data/
cp -r assets/* /data/assets/

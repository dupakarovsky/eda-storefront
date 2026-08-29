#!/usr/bin/env bash
set -euo pipefail

BASE_URL="http://localhost:8080/stores.v1.StoresService"
HEADERS=(-H "Content-Type: application/json" -H "Connect-Protocol-Version: 1")

echo "== CreateStore =="
CREATE_STORE_RESP=$(curl -s "${HEADERS[@]}" -d '{"name":"Verification Grocer","location":"1 Verify Ave"}' "$BASE_URL/CreateStore")
echo "$CREATE_STORE_RESP" | jq .
STORE_ID=$(echo "$CREATE_STORE_RESP" | jq -r '.store.id')

echo "== RebrandStore =="
curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"Verification Grocer Renamed\"}" "$BASE_URL/RebrandStore" | jq .

echo "== EnableStoreParticipation =="
curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\"}" "$BASE_URL/EnableStoreParticipation" | jq .

echo "== GetStore (after enable) =="
curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\"}" "$BASE_URL/GetStore" | jq .

echo "== DisableStoreParticipation =="
curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\"}" "$BASE_URL/DisableStoreParticipation" | jq .

echo "== AddProduct =="
ADD_PRODUCT_RESP=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"Verification Widget\",\"description\":\"a test product\",\"sku\":\"SKU-VERIFY-1\",\"price\":9.99}" "$BASE_URL/AddProduct")
echo "$ADD_PRODUCT_RESP" | jq .
PRODUCT_ID=$(echo "$ADD_PRODUCT_RESP" | jq -r '.product.id')

echo "== RebrandProduct =="
curl -s "${HEADERS[@]}" -d "{\"productId\":\"$PRODUCT_ID\",\"name\":\"Verification Widget Deluxe\",\"description\":\"an upgraded test product\"}" "$BASE_URL/RebrandProduct" | jq .

echo "== IncreaseProductPrice =="
curl -s "${HEADERS[@]}" -d "{\"productId\":\"$PRODUCT_ID\",\"price\":12.99}" "$BASE_URL/IncreaseProductPrice" | jq .

echo "== DecreaseProductPrice =="
curl -s "${HEADERS[@]}" -d "{\"productId\":\"$PRODUCT_ID\",\"price\":10.99}" "$BASE_URL/DecreaseProductPrice" | jq .

echo "== ListProducts =="
curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\"}" "$BASE_URL/ListProducts" | jq .

echo "== RemoveProduct =="
curl -s "${HEADERS[@]}" -d "{\"productId\":\"$PRODUCT_ID\"}" "$BASE_URL/RemoveProduct" | jq .

echo "== ListProducts (after removal, expect empty array) =="
curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\"}" "$BASE_URL/ListProducts" | jq .

echo "== GetStore against unknown ID (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"storeId":"does-not-exist"}' "$BASE_URL/GetStore"

echo "== RebrandStore with blank name (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"\"}" "$BASE_URL/RebrandStore"

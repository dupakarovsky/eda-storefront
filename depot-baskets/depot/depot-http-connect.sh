#!/usr/bin/env bash
set -euo pipefail

STORES_URL="http://localhost:8080/stores.v1.StoresService"
DEPOT_URL="http://localhost:8082/depot.v1.DepotService"
HEADERS=(-H "Content-Type: application/json" -H "Connect-Protocol-Version: 1")

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "== [stores] CreateStore (store A) =="
CREATE_STORE_A_RESP=$(curl -s "${HEADERS[@]}" -d '{"name":"Depot Verification Grocer","location":"1 Verify Ave"}' "$STORES_URL/CreateStore")
echo "$CREATE_STORE_A_RESP" | jq .
STORE_A_ID=$(echo "$CREATE_STORE_A_RESP" | jq -r '.store.id')

echo "== [stores] CreateStore (store B) =="
CREATE_STORE_B_RESP=$(curl -s "${HEADERS[@]}" -d '{"name":"Depot Verification Hardware","location":"2 Verify Ave"}' "$STORES_URL/CreateStore")
echo "$CREATE_STORE_B_RESP" | jq .
STORE_B_ID=$(echo "$CREATE_STORE_B_RESP" | jq -r '.store.id')

echo "== [stores] AddProduct (store A, product 1) =="
ADD_PRODUCT_A1_RESP=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_A_ID\",\"name\":\"Sourdough Loaf\",\"description\":\"Fresh baked\",\"sku\":\"SKU-DEPOT-VERIFY-1\",\"price\":5.00}" "$STORES_URL/AddProduct")
echo "$ADD_PRODUCT_A1_RESP" | jq .
PRODUCT_A1_ID=$(echo "$ADD_PRODUCT_A1_RESP" | jq -r '.product.id')

echo "== [stores] AddProduct (store A, product 2) =="
ADD_PRODUCT_A2_RESP=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_A_ID\",\"name\":\"Oat Milk\",\"description\":\"1 quart\",\"sku\":\"SKU-DEPOT-VERIFY-2\",\"price\":3.25}" "$STORES_URL/AddProduct")
echo "$ADD_PRODUCT_A2_RESP" | jq .
PRODUCT_A2_ID=$(echo "$ADD_PRODUCT_A2_RESP" | jq -r '.product.id')

echo "== [stores] AddProduct (store B, product 1) =="
ADD_PRODUCT_B1_RESP=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_B_ID\",\"name\":\"Hammer\",\"description\":\"16oz claw\",\"sku\":\"SKU-DEPOT-VERIFY-3\",\"price\":12.00}" "$STORES_URL/AddProduct")
echo "$ADD_PRODUCT_B1_RESP" | jq .
PRODUCT_B1_ID=$(echo "$ADD_PRODUCT_B1_RESP" | jq -r '.product.id')

echo "== waiting for depot's projector to consume stores' events =="
sleep 2

echo "== [depot] catalog_stores/catalog_products cache check (both stores, all three products expected) =="
set -a
source "$REPO_ROOT/.postgres.env"
set +a
docker compose -f "$REPO_ROOT/docker-compose.yaml" exec -T -e PGPASSWORD="$EDA_DEV_PASSWORD" postgres \
    psql -h 127.0.0.1 -U eda_dev -d depotdb -c "SELECT id, name FROM catalog_stores WHERE id IN ('$STORE_A_ID','$STORE_B_ID');"
docker compose -f "$REPO_ROOT/docker-compose.yaml" exec -T -e PGPASSWORD="$EDA_DEV_PASSWORD" postgres \
    psql -h 127.0.0.1 -U eda_dev -d depotdb -c "SELECT id, store_id, name FROM catalog_products WHERE id IN ('$PRODUCT_A1_ID','$PRODUCT_A2_ID','$PRODUCT_B1_ID');"

echo "== [depot] CreateShoppingList =="
CREATE_LIST_RESP=$(curl -s "${HEADERS[@]}" -d '{"orderId":"order-verify-001"}' "$DEPOT_URL/CreateShoppingList")
echo "$CREATE_LIST_RESP" | jq .
LIST_ID=$(echo "$CREATE_LIST_RESP" | jq -r '.shoppingList.id')
echo "$CREATE_LIST_RESP" | jq -e '.shoppingList.orderId == "order-verify-001"' >/dev/null && echo "orderId round-trip OK" || echo "orderId MISMATCH"

echo "== [depot] AddItem (store A, product 1) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\",\"storeId\":\"$STORE_A_ID\",\"productId\":\"$PRODUCT_A1_ID\"}" "$DEPOT_URL/AddItem" | jq .

echo "== [depot] AddItem (store A, product 1 again, expect quantity 2) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\",\"storeId\":\"$STORE_A_ID\",\"productId\":\"$PRODUCT_A1_ID\"}" "$DEPOT_URL/AddItem" | jq .

echo "== [depot] AddItem (store A, product 2, expect second item under same stop) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\",\"storeId\":\"$STORE_A_ID\",\"productId\":\"$PRODUCT_A2_ID\"}" "$DEPOT_URL/AddItem" | jq .

echo "== [depot] AddItem (store B, product 1, expect a second stop) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\",\"storeId\":\"$STORE_B_ID\",\"productId\":\"$PRODUCT_B1_ID\"}" "$DEPOT_URL/AddItem" | jq .

echo "== [depot] GetShoppingList (expect 2 stops: store A with 2 items, store B with 1 item) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\"}" "$DEPOT_URL/GetShoppingList" | jq .

echo "== [depot] RemoveItem (store A, product 1, expect quantity to drop to 1, not disappear) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\",\"storeId\":\"$STORE_A_ID\",\"productId\":\"$PRODUCT_A1_ID\"}" "$DEPOT_URL/RemoveItem" | jq .

echo "== [depot] RemoveItem (store B, product 1, expect store B's stop to disappear entirely) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\",\"storeId\":\"$STORE_B_ID\",\"productId\":\"$PRODUCT_B1_ID\"}" "$DEPOT_URL/RemoveItem" | jq .

echo "== [depot] GetShoppingList (expect only store A's stop remains, 2 items) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\"}" "$DEPOT_URL/GetShoppingList" | jq .

echo "== [depot] InitiateShoppingList =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\"}" "$DEPOT_URL/InitiateShoppingList" | jq .

echo "== [depot] AddItem after Initiate (expect InvalidArgument / HTTP 400, contents frozen) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\",\"storeId\":\"$STORE_A_ID\",\"productId\":\"$PRODUCT_A2_ID\"}" "$DEPOT_URL/AddItem"

echo "== [depot] CompleteShoppingList =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\"}" "$DEPOT_URL/CompleteShoppingList" | jq .

echo "== [depot] CancelShoppingList on already-completed list (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST_ID\"}" "$DEPOT_URL/CancelShoppingList"

echo "== [depot] AddItem against a store depot never cached (expect NotFound / HTTP 404) =="
NEW_LIST_RESP=$(curl -s "${HEADERS[@]}" -d '{}' "$DEPOT_URL/CreateShoppingList")
NEW_LIST_ID=$(echo "$NEW_LIST_RESP" | jq -r '.shoppingList.id')
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"shoppingListId\":\"$NEW_LIST_ID\",\"storeId\":\"does-not-exist\",\"productId\":\"does-not-exist\"}" "$DEPOT_URL/AddItem"

echo "== [depot] AddItem against a valid store but a product it never carries (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"shoppingListId\":\"$NEW_LIST_ID\",\"storeId\":\"$STORE_A_ID\",\"productId\":\"does-not-exist\"}" "$DEPOT_URL/AddItem"

echo "== [depot] CompleteShoppingList on an empty (never-initiated) list (expect InvalidArgument / HTTP 400, not-active guard fires first) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"shoppingListId\":\"$NEW_LIST_ID\"}" "$DEPOT_URL/CompleteShoppingList"

echo "== [depot] GetShoppingList against unknown ID (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"shoppingListId":"does-not-exist"}' "$DEPOT_URL/GetShoppingList"

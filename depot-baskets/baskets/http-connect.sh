#!/usr/bin/env bash
set -euo pipefail

STORES_URL="http://localhost:8080/stores.v1.StoresService"
BASKETS_URL="http://localhost:8081/baskets.v1.BasketsService"
HEADERS=(-H "Content-Type: application/json" -H "Connect-Protocol-Version: 1")

echo "== [stores] CreateStore =="
CREATE_STORE_RESP=$(curl -s "${HEADERS[@]}" -d '{"name":"Basket Verification Grocer","location":"1 Verify Ave"}' "$STORES_URL/CreateStore")
echo "$CREATE_STORE_RESP" | jq .
STORE_ID=$(echo "$CREATE_STORE_RESP" | jq -r '.store.id')

echo "== [stores] AddProduct (first item) =="
ADD_PRODUCT_RESP=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"Sourdough Loaf\",\"description\":\"Fresh baked\",\"sku\":\"SKU-BASKET-VERIFY-1\",\"price\":5.00}" "$STORES_URL/AddProduct")
echo "$ADD_PRODUCT_RESP" | jq .
PRODUCT_ID=$(echo "$ADD_PRODUCT_RESP" | jq -r '.product.id')

echo "== [stores] AddProduct (second item, for multi-item basket coverage) =="
ADD_PRODUCT_RESP_2=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"Oat Milk\",\"description\":\"1 quart\",\"sku\":\"SKU-BASKET-VERIFY-2\",\"price\":3.25}" "$STORES_URL/AddProduct")
echo "$ADD_PRODUCT_RESP_2" | jq .
PRODUCT_ID_2=$(echo "$ADD_PRODUCT_RESP_2" | jq -r '.product.id')

# baskets consumes the the messages from the stream and pupulates the
# catalog_product table with the entries for these products
echo "== waiting for baskets' projector to consume ProductAdded off stores' stream =="
sleep 2

# basket service creates a new basket
echo "== [baskets] StartBasket =="
START_BASKET_RESP=$(curl -s "${HEADERS[@]}" -d '{}' "$BASKETS_URL/StartBasket")
echo "$START_BASKET_RESP" | jq .
BASKET_ID=$(echo "$START_BASKET_RESP" | jq -r '.basket.id')

# basket adds an product #1 (qtn:1)
echo "== [baskets] AddItem (product 1) =="
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\",\"productId\":\"$PRODUCT_ID\"}" "$BASKETS_URL/AddItem" | jq .

# basket adds an product #1 again (qnt: 2)
echo "== [baskets] AddItem (product 1 again, expect quantity 2) =="
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\",\"productId\":\"$PRODUCT_ID\"}" "$BASKETS_URL/AddItem" | jq .

# basket adds an product #2 (qtn:1)
echo "== [baskets] AddItem (product 2) =="
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\",\"productId\":\"$PRODUCT_ID_2\"}" "$BASKETS_URL/AddItem" | jq .

echo "== [baskets] GetBasket (expect 2 items, quantities 2 and 1) =="
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\"}" "$BASKETS_URL/GetBasket" | jq .

echo "== [baskets] RemoveItem (product 1, expect quantity to drop to 1, not disappear) =="
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\",\"productId\":\"$PRODUCT_ID\"}" "$BASKETS_URL/RemoveItem" | jq .

echo "== [baskets] Checkout =="
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\"}" "$BASKETS_URL/Checkout" | jq .

echo "== [baskets] CancelBasket on already-checked-out basket (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\"}" "$BASKETS_URL/CancelBasket"

echo "== [baskets] AddItem against a product stores never created (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\",\"productId\":\"does-not-exist\"}" "$BASKETS_URL/AddItem"

echo "== [baskets] GetBasket against unknown ID (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"basketId":"does-not-exist"}' "$BASKETS_URL/GetBasket"

echo "== [baskets] AddItem with blank productId (expect InvalidArgument / HTTP 400) =="
NEW_BASKET_RESP=$(curl -s "${HEADERS[@]}" -d '{}' "$BASKETS_URL/StartBasket")
NEW_BASKET_ID=$(echo "$NEW_BASKET_RESP" | jq -r '.basket.id')
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"basketId\":\"$NEW_BASKET_ID\",\"productId\":\"\"}" "$BASKETS_URL/AddItem"

echo "== [baskets] Checkout on empty basket (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"basketId\":\"$NEW_BASKET_ID\"}" "$BASKETS_URL/Checkout"

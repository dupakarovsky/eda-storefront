#!/usr/bin/env bash
# ordering-http-connect.sh
set -euo pipefail

STORES_URL="http://localhost:8080/stores.v1.StoresService"
BASKETS_URL="http://localhost:8081/baskets.v1.BasketsService"
HEADERS=(-H "Content-Type: application/json" -H "Connect-Protocol-Version: 1")

echo "== [stores] CreateStore =="
CREATE_STORE_RESP=$(curl -s "${HEADERS[@]}" -d '{"name":"Ordering Verification Grocer","location":"1 Verify Ave"}' "$STORES_URL/CreateStore")
echo "$CREATE_STORE_RESP" | jq .
STORE_ID=$(echo "$CREATE_STORE_RESP" | jq -r '.store.id')

echo "== [stores] AddProduct =="
ADD_PRODUCT_RESP=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"Verification Widget\",\"description\":\"a test product\",\"sku\":\"SKU-ORDERING-VERIFY-1\",\"price\":9.99}" "$STORES_URL/AddProduct")
echo "$ADD_PRODUCT_RESP" | jq .
PRODUCT_ID=$(echo "$ADD_PRODUCT_RESP" | jq -r '.product.id')

echo "== waiting for baskets' AND depot's projectors to consume ProductAdded off stores' stream =="
sleep 2

echo "== [baskets] StartBasket =="
START_BASKET_RESP=$(curl -s "${HEADERS[@]}" -d '{}' "$BASKETS_URL/StartBasket")
echo "$START_BASKET_RESP" | jq .
BASKET_ID=$(echo "$START_BASKET_RESP" | jq -r '.basket.id')

echo "== [baskets] AddItem =="
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\",\"productId\":\"$PRODUCT_ID\"}" "$BASKETS_URL/AddItem" | jq .

echo "== [baskets] Checkout (this publishes BasketCheckedOut, which kicks off ordering) =="
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET_ID\"}" "$BASKETS_URL/Checkout" | jq .

echo ""
echo "== Basket checked out. Basket ID (== Order ID, per ordering's own convention): $BASKET_ID =="
echo "Watch the ordering logs now for OrderCreated + the depot orchestration calls, then run:"
echo "  make orderingdb/dev"
echo "  SELECT id, status, shopping_list_id FROM orders WHERE id = '$BASKET_ID';"
echo "to get the shopping_list_id before continuing to the depot step."

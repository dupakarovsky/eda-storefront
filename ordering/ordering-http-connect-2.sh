#!/usr/bin/env bash
set -euo pipefail

STORES_URL="http://localhost:8080/stores.v1.StoresService"
BASKETS_URL="http://localhost:8081/baskets.v1.BasketsService"
DEPOT_URL="http://localhost:8082/depot.v1.DepotService"
ORDERING_URL="http://localhost:8083/ordering.v1.OrderingService"
HEADERS=(-H "Content-Type: application/json" -H "Connect-Protocol-Version: 1")

echo "== [stores] CreateStore =="
CREATE_STORE_RESP=$(curl -s "${HEADERS[@]}" -d '{"name":"Ordering Verification Grocer","location":"1 Verify Ave"}' "$STORES_URL/CreateStore")
STORE_ID=$(echo "$CREATE_STORE_RESP" | jq -r '.store.id')
echo "$CREATE_STORE_RESP" | jq .

echo "== [stores] AddProduct =="
ADD_PRODUCT_RESP=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"Verification Widget\",\"description\":\"a test product\",\"sku\":\"SKU-ORDERING-VERIFY-2\",\"price\":9.99}" "$STORES_URL/AddProduct")
PRODUCT_ID=$(echo "$ADD_PRODUCT_RESP" | jq -r '.product.id')
echo "$ADD_PRODUCT_RESP" | jq .

echo "== waiting for baskets' and depot's projectors to consume ProductAdded =="
sleep 2

echo "== [baskets] StartBasket, AddItem, Checkout (order #1 — will run to completion) =="
BASKET1_ID=$(curl -s "${HEADERS[@]}" -d '{}' "$BASKETS_URL/StartBasket" | jq -r '.basket.id')
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET1_ID\",\"productId\":\"$PRODUCT_ID\"}" "$BASKETS_URL/AddItem" | jq .
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET1_ID\"}" "$BASKETS_URL/Checkout" | jq .

echo "== waiting for ordering to consume BasketCheckedOut and orchestrate depot =="
sleep 3

echo "== [ordering] GetOrder (order #1, expect status 'created', shoppingListId set) =="
GET_ORDER1_RESP=$(curl -s "${HEADERS[@]}" -d "{\"orderId\":\"$BASKET1_ID\"}" "$ORDERING_URL/GetOrder")
echo "$GET_ORDER1_RESP" | jq .
LIST1_ID=$(echo "$GET_ORDER1_RESP" | jq -r '.order.shoppingListId')

echo "== [depot] InitiateShoppingList + CompleteShoppingList (order #1's list) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST1_ID\"}" "$DEPOT_URL/InitiateShoppingList" | jq .
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST1_ID\"}" "$DEPOT_URL/CompleteShoppingList" | jq .

echo "== waiting for ordering to consume ShoppingListCompleted =="
sleep 3

echo "== [ordering] GetOrder (order #1, expect status 'readied') =="
curl -s "${HEADERS[@]}" -d "{\"orderId\":\"$BASKET1_ID\"}" "$ORDERING_URL/GetOrder" | jq .

echo "== [ordering] CancelOrder on an already-readied order (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"orderId\":\"$BASKET1_ID\"}" "$ORDERING_URL/CancelOrder"

echo "== [baskets] StartBasket, AddItem, Checkout (order #2 — will be canceled mid-flight) =="
BASKET2_ID=$(curl -s "${HEADERS[@]}" -d '{}' "$BASKETS_URL/StartBasket" | jq -r '.basket.id')
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET2_ID\",\"productId\":\"$PRODUCT_ID\"}" "$BASKETS_URL/AddItem" | jq .
curl -s "${HEADERS[@]}" -d "{\"basketId\":\"$BASKET2_ID\"}" "$BASKETS_URL/Checkout" | jq .

echo "== waiting for ordering to consume BasketCheckedOut and orchestrate depot (order #2) =="
sleep 3

echo "== [ordering] GetOrder (order #2, expect status 'created') =="
GET_ORDER2_RESP=$(curl -s "${HEADERS[@]}" -d "{\"orderId\":\"$BASKET2_ID\"}" "$ORDERING_URL/GetOrder")
echo "$GET_ORDER2_RESP" | jq .
LIST2_ID=$(echo "$GET_ORDER2_RESP" | jq -r '.order.shoppingListId')

echo "== [ordering] CancelOrder (order #2, expect status 'canceled') =="
curl -s "${HEADERS[@]}" -d "{\"orderId\":\"$BASKET2_ID\"}" "$ORDERING_URL/CancelOrder" | jq .

echo "== [depot] GetShoppingList (order #2's list, expect status 'canceled' — confirms ordering's depot call took effect) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$LIST2_ID\"}" "$DEPOT_URL/GetShoppingList" | jq .

echo "== [ordering] CancelOrder on an already-canceled order (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"orderId\":\"$BASKET2_ID\"}" "$ORDERING_URL/CancelOrder"

echo "== [ordering] GetOrder against unknown ID (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"orderId":"does-not-exist"}' "$ORDERING_URL/GetOrder"

echo "== [ordering] CancelOrder against unknown ID (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"orderId":"does-not-exist"}' "$ORDERING_URL/CancelOrder"

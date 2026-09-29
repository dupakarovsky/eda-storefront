#!/usr/bin/env bash
# ordering-complete-shopping-list.sh
set -euo pipefail

DEPOT_URL="http://localhost:8082/depot.v1.DepotService"
HEADERS=(-H "Content-Type: application/json" -H "Connect-Protocol-Version: 1")

SHOPPING_LIST_ID="18360c92-24a3-486e-bb85-04dcbd3c9b83"

echo "== [depot] InitiateShoppingList =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$SHOPPING_LIST_ID\"}" "$DEPOT_URL/InitiateShoppingList" | jq .

echo "== [depot] CompleteShoppingList (this publishes ShoppingListCompleted, which ordering's depot subscriber should pick up) =="
curl -s "${HEADERS[@]}" -d "{\"shoppingListId\":\"$SHOPPING_LIST_ID\"}" "$DEPOT_URL/CompleteShoppingList" | jq .

echo ""
echo "Watch the ordering logs now for CompleteOrder.Execute + the OrderReadied publish, then run:"
echo "  make orderingdb/dev"
echo "  SELECT id, status, shopping_list_id FROM orders WHERE id = 'e6a75226-04d5-4877-bef6-883729ec7bcf';"
echo "status should now read 'readied'."

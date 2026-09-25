package depotclient

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/depot/depotpb"
	"github.com/dupakarovsky/eda-storefront/depot/depotpb/depotpbconnect"
)

// Config holds the URL for communicating via RPC to the depot service
type Config struct {
	URL string
}

// Client defines a rpc client need to connect with the "depot" service
type Client struct {
	rpc depotpbconnect.DepotServiceClient
}

// New construct a new RPC Client
func New(cfg Config) *Client {
	// New uses Go's http.Client to communicate with `depot` by dialing
	// to it's http address
	return &Client{
		rpc: depotpbconnect.NewDepotServiceClient(http.DefaultClient, cfg.URL),
	}
}

// CreateShoppingList makes a rpc Request to `depot` for creating a ShoppingList and
// by sending a Request recieves back a response
func (c *Client) CreateShoppingList(ctx context.Context, orderID string) (string, error) {
	// create a rpc Request to the `depot` servcice requesting the creation of a ShoppingList.
	// while passing the CreateShoppingListRequest with the orderID
	req := connect.NewRequest(&depotpb.CreateShoppingListRequest{OrderId: orderID})

	// receive the CreateShoppingListReponse back
	resp, err := c.rpc.CreateShoppingList(ctx, req)
	if err != nil {
		return "", fmt.Errorf("creating shopping list for order %q: %w", orderID, err)
	}

	// return the created ShoppingList id
	return resp.Msg.ShoppingList.Id, nil
}

// AddItem make a rpc request to the `depot` service by providing a Request
func (c *Client) AddItem(ctx context.Context, shoppingListID, storeID, productID string) error {

	// define the rpc request to `depot`
	req := connect.NewRequest(&depotpb.AddItemRequest{
		ShoppingListId: shoppingListID,
		StoreId:        storeID,
		ProductId:      productID,
	})

	// make the call and receive back a AddItemResponse
	if _, err := c.rpc.AddItem(ctx, req); err != nil {
		return fmt.Errorf("adding product %q to shopping list %q: %w", productID, shoppingListID, err)
	}

	return nil
}

// GetShoppingList will make an rpc call to the `depot` service /GetShoppingList endpoint an retrieve
// a shopping list with all of it's Stops. For each stop we'll get the items and associate each
// product ID to it's quantity
func (c *Client) GetShoppingList(ctx context.Context, shoppingListID string) (map[string]int, error) {

	// make a new rpc Request for a ShoppingList
	req := connect.NewRequest(&depotpb.GetShoppingListRequest{
		ShoppingListId: shoppingListID,
	})

	// make a rpc request to retrieve the shopping list associated with this ID to the GetShoppingList endpoint
	// we should get a GetShoppingListResponst which contains a ShoppingList struct
	resp, err := c.rpc.GetShoppingList(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("getting shopping list %q: %w", shoppingListID, err)
	}

	// now we'll make a map to keying each productID to it's quantity
	quantities := make(map[string]int)

	// access each Stop
	for _, stop := range resp.Msg.ShoppingList.Stops {
		// from each Stop, get it's Items
		for _, item := range stop.Items {
			// link each product to it's quanityt
			quantities[item.ProductId] = int(item.Quantity)
		}
	}

	return quantities, nil
}

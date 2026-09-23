// ordering/internal/baskets/baskets.go
package baskets

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/application"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// basketCheckedOutEnvelope takes the envelope type that'll be
// extracted from the stream
type basketCheckedOutEnvelope struct {
	AggregateID string          `json:"aggregate_id"`
	EventName   string          `json:"event_name"`
	Data        json.RawMessage `json:"data"`
}

// basketCheckedOutItemPayload is a single Item from the Data
// field
type basketCheckedOutItemPayload struct {
	ProductID string `json:"product_id"`
	StoreID   string `json:"store_id"`
	Quantity  int    `json:"quantity"`
}

// basketCheckedOutPayload is the type will contain the
// data extracted from the basketCheckedOutItemPayload.Data field
type basketCheckedOutPayload struct {
	Items []basketCheckedOutItemPayload `json:"items"`
}

// Handler defines a handler that will to a new message being published to the baskets stream.
// When a message arrives, the handler will run, extract the JSON blob from the streasm Message.Data field
// and unwrapp it. It'll verify the type for BasketCheckoutPayload and call the CreateOrder use case to
// generate a new Order
func Handler(createOrderUC *application.CreateOrder, logger *slog.Logger) foundationnats.MessageHandler {

	handler := func(ctx context.Context, msg jetstream.Msg) error {

		var env basketCheckedOutEnvelope
		if err := json.Unmarshal(msg.Data(), &env); err != nil {
			return fmt.Errorf("unmarshaling envelope: %w", err)
		}

		if env.EventName != "BasketCheckedOut" {
			logger.Warn("no handler mapping for event, skipping", "event_name", env.EventName)
			return nil
		}

		var payload basketCheckedOutPayload
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			return fmt.Errorf("unmarshaling BasketCheckedOut payload: %w", err)
		}

		items := make([]domain.Item, 0, len(payload.Items))
		for _, item := range payload.Items {
			items = append(items, domain.Item{
				ProductID: item.ProductID,
				StoreID:   item.StoreID,
				Quantity:  item.Quantity,
			})
		}

		id := uuid.NewString()

		if _, err := createOrderUC.Execute(ctx, id, items); err != nil {
			return fmt.Errorf("creating order: %w", err)
		}

		return nil
	}

	return handler
}

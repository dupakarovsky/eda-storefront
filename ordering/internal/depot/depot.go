package depot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/application"
	"github.com/nats-io/nats.go/jetstream"
)

// depotShoppingListCompletedEnvelope defines a type that will
// contain data extracted from the orders jetstream
type depotShoppingListCompletedEnvelope struct {
	AggregateID string          `json:"aggregate_id"`
	EventName   string          `json:"event_name"`
	Data        json.RawMessage `json:"data"`
}

// Handler defines a handler that will react to a new message being published to the depot stream.
// When `depot` publishes an Event to it's stream this service Subscriber will call this handler.
// Then handler will extract the JSON blob from the stream's Message.Data field and unwrapp it.
// Then, It'll verify the type for ShoppingListCompleted and call the CompleteOrder use case to
// generate a new Order
func Handler(completeOrderUC *application.CompleteOrder, logger *slog.Logger) foundationnats.MessageHandler {

	handler := func(ctx context.Context, msg jetstream.Msg) error {

		// env will hold the wrapped Event type from the jetstream Message
		var env depotShoppingListCompletedEnvelope

		// unwramp the message Data into the type
		if err := json.Unmarshal(msg.Data(), &env); err != nil {
			return fmt.Errorf("unmarshaling envelope: %w", err)
		}

		// verify the that Event type is actually ShoppingListCompleted by it's EventName,
		// since that's the only event we care about.
		if env.EventName != "ShoppingListCompleted" {
			logger.Warn("no handler mapping for event, skipping", "event_name", env.EventName)
			return nil
		}

		// ShoppingListCompleted does not carry a payload, so nothing
		// will be in the envelope's Data propery.
		// -----------------------------------------------------------

		// call the completeOrderUC use case passing the AggregateID (shopping list id)
		// to complete the order
		_, err := completeOrderUC.Execute(ctx, env.AggregateID)
		if err != nil {
			return fmt.Errorf("completing order: %w", err)
		}

		return nil
	}

	return handler
}

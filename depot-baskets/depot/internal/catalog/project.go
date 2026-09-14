package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/nats-io/nats.go/jetstream"
)

// envelope will be used by the Projector method which is responsible for extracting
// the events from the stream in their CONTRACT form and converting them into one of
// the concrete data types below that can be inserted into the repository
type envelope struct {
	AggregateID string          `json:"aggregate_id"`
	EventName   string          `json:"event_name"`
	Data        json.RawMessage `json:"data"`
}

//----------------------------------
// === Store's Related Payloads ===
//----------------------------------

// storeCreatedPayload holds data comming a StoreCreatedPayload published
// by "stores" to it's stream
type storeCreatedPayload struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

// storeRebrandedPayload holds data comming form StoreRebrandedPayload
// published by "stores" to it's stream
type storeRebrandedPayload struct {
	Name string `json:"name"`
}

// storeParticipationPayload holds data comming from a StoreParticipationPayload
// type published by "stores" to it's stream
type storeParticipationPayload struct {
	Participating bool `json:"participating"`
}

//------------------------------------
// ===  Product's Related Payloads ===
//------------------------------------

// productAddedPayload holds data comming from a ProductAddedPayload
// published the "stores"  to it's stream
type productAddedPayload struct {
	StoreID     string  `json:"store_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	SKU         string  `json:"sku"`
	Price       float64 `json:"price"`
}

// productAddedPayload holds data comming from ProductRebrandedPayload
// published by "stores" to it's stream
type productRebrandedPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// productPriceDeltaPayload holds data comming from ProductPriceDeltaPayload
// published by "stores" to it's stream
type productPriceDeltaPayload struct {
	Delta float64 `json:"delta"`
}

// ----------------------------------------------------------------------------

// Project will be extract the stream message, take the Data property and decode it
// to envelope type. From there it'll evaluate the EventName field to find out
// what event that originated that message.
func Project(repo Repository, logger *slog.Logger) foundationnats.MessageHandler {

	// returns the handler function that will take a jetstream Mesesage Data property,
	// which contains the JSON data from the concrete Payload type published either
	// by the "Store" or "Product" aggregates, and unmashal it into an local
	// envelope. It'll then evaluate the EventName to find out which event was
	// published. If it's a 'Store' aggregate event it'll pupulate the catalog_store
	// table. If it's an 'Product' aggreate event, it'll populate the catalgo_product
	// table
	handler := func(ctx context.Context, msg jetstream.Msg) error {

		var env envelope

		// unmashal the Data property of the message in the stream to the envelope
		// data structure
		if err := json.Unmarshal(msg.Data(), &env); err != nil {
			return fmt.Errorf("unmarshaling envelope: %w", err)
		}

		// now we need to know the EventName to convert the envelope struct into on
		// of the concrete paylaod types above that can be used by the repository
		switch env.EventName {

		case "StoreCreated":
			var p storeCreatedPayload
			if err := json.Unmarshal(env.Data, &p); err != nil {
				return fmt.Errorf("unmarshaling StoreCreated payload: %w", err)
			}
			return repo.CreateStore(ctx, env.AggregateID, p.Name, p.Location)

		case "StoreRebranded":
			var p storeRebrandedPayload
			if err := json.Unmarshal(env.Data, &p); err != nil {
				return fmt.Errorf("unmarshaling StoreRebranded payload: %w", err)
			}
			return repo.RenameStore(ctx, env.AggregateID, p.Name)

		case "StoreParticipationEnabled", "StoreParticipationDisabled":
			var p storeParticipationPayload
			if err := json.Unmarshal(env.Data, &p); err != nil {
				return fmt.Errorf("unmarshaling %s payload: %w", env.EventName, err)
			}
			return repo.SetStoreParticipation(ctx, env.AggregateID, p.Participating)

		case "ProductAdded":
			var p productAddedPayload
			if err := json.Unmarshal(env.Data, &p); err != nil {
				return fmt.Errorf("unmarshaling ProductAdded payload: %w", err)
			}
			return repo.CreateProduct(ctx, env.AggregateID, p.StoreID, p.Name, p.Description, p.SKU, p.Price)

		case "ProductRebranded":
			var p productRebrandedPayload
			if err := json.Unmarshal(env.Data, &p); err != nil {
				return fmt.Errorf("unmarshaling ProductRebranded payload: %w", err)
			}
			return repo.RebrandProduct(ctx, env.AggregateID, p.Name, p.Description)

		case "ProductPriceIncreased", "ProductPriceDecreased":
			var p productPriceDeltaPayload
			if err := json.Unmarshal(env.Data, &p); err != nil {
				return fmt.Errorf("unmarshaling %s payload: %w", env.EventName, err)
			}

			meta, err := msg.Metadata()
			if err != nil {
				return fmt.Errorf("reading message metadata: %w", err)
			}

			err = repo.AdjustProductPrice(ctx, env.AggregateID, p.Delta, meta.Sequence.Stream)
			if err != nil && err != ErrPriceAdjustmentSkipped {
				return err
			}
			if err == ErrPriceAdjustmentSkipped {
				logger.Info("price adjustment skipped, already applied",
					"product_id", env.AggregateID,
					"stream_seq", meta.Sequence.Stream,
				)
			}
			return nil

		case "ProductRemoved":
			return repo.DeleteProduct(ctx, env.AggregateID)

		default:
			logger.Warn("no projection mapping for event, skipping", "event_name", env.EventName)
			return nil
		}
	}

	return handler
}

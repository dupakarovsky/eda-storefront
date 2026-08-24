package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// CreateStore is a temporary construct to connecting to the database repository
// and publishing and Event to nats.
type CreateStore struct {
	storeRepo domain.StoreRepository
	publisher *foundationnats.Publisher
	logger    *slog.Logger
}

// NewCreateStore will construct a new CreateStore struct
func NewCreateStore(storeRepo domain.StoreRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *CreateStore {
	return &CreateStore{
		storeRepo: storeRepo,
		publisher: publisher,
		logger:    logger,
	}
}

// Execute will perform the action of creating a new Store from a id, name an location. It'll persist
// the creation in the store repository and then publish an event to NATs
func (uc *CreateStore) Execute(ctx context.Context, id, name, location string) (*domain.Store, error) {

	// first, construct a new Store from the parameters given
	store, event, err := domain.NewStore(id, name, location)
	if err != nil {
		return nil, fmt.Errorf("constructing store: %w", err)
	}

	// store and event received
	// ------------------------

	// save the new Store the repository
	if err := uc.storeRepo.Save(ctx, store); err != nil {
		return nil, fmt.Errorf("saving store: %w", err)
	}

	//INFO:: If publishDomainEvent fails, the store remains created in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	// publishe the event
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return store, err
	}

	// store saved, prepare event for publishing
	// -----------------------------------------

	// prepare the event for transport
	// envelope, err := storesevents.NewEnvelope(event)
	// if err != nil {
	// 	return nil, fmt.Errorf("building envelope: %w", err)
	// }
	//
	// // marshal the envelope into JSON bytes
	// payload, err := envelope.Marshal()
	// if err != nil {
	// 	return nil, fmt.Errorf("marshaling envelope: %w", err)
	// }
	//
	// // ready the subject to publish under the NATS stream
	// subject, err := storesevents.SubjectFor(event)
	// if err != nil {
	// 	return nil, fmt.Errorf("resolving subject: %w", err)
	// }
	//
	// // publish the event payload to NATS using the retry-logic
	// ack, err := publishWithRetry(ctx, uc.publisher, uc.logger, subject, payload)
	// if err != nil {
	// 	uc.logger.Error("event publish exhausted retries; store saved but event not delivered",
	// 		"aggregate_id", event.AggregateID(),
	// 		"event_name", event.EventName(),
	// 		"subject", subject,
	// 		"error", err,
	// 	)
	//
	// 	return store, fmt.Errorf("publishing event: %w", err)
	// }
	//
	// event successfuly published
	// ---------------------------
	//
	// event published. emit a log
	// uc.logger.Info("event puslished", "subject", subject, "stream", ack.Stream, "seq", ack.Sequence)

	// return the Store with no errors
	return store, nil
}

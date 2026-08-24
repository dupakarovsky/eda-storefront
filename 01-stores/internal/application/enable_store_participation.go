package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// EnableStoreParticipation defines a struct type that is able to cummunicate
// with the repository and publisher to persist a Store's enable particitpation action
type EnableStoreParticipation struct {
	storeRepo domain.StoreRepository
	publisher *foundationnats.Publisher
	logger    *slog.Logger
}

// NewEnableStoreParticipation creates a new EnableStoreParticipation struct
func NewEnableStoreParticipation(storeRepo domain.StoreRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *EnableStoreParticipation {
	return &EnableStoreParticipation{
		storeRepo: storeRepo,
		publisher: publisher,
		logger:    logger,
	}
}

// Execute will connect to the database to persist the Store's participation change, then
// publish the event to NATS
func (uc *EnableStoreParticipation) Execute(ctx context.Context, storeId string) (*domain.Store, error) {

	// locate the store that needs enabling participation
	store, err := uc.storeRepo.FindByID(ctx, storeId)
	if err != nil {
		return nil, fmt.Errorf("finding store: %w", err)
	}

	// store located. enable the participation.
	// ---------------------------------------

	// emit the enable particiatpion event
	event, err := store.EnableParticipation()
	if err != nil {
		return nil, fmt.Errorf("enableling participation: %w", err)
	}

	// update the store to the repository

	err = uc.storeRepo.Save(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("saving store: %w", err)
	}

	//INFO:: If publishDomainEvent fails, the store remains created in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	// event emited. publish it
	err = publishDomainEvent(ctx, uc.publisher, uc.logger, event)
	if err != nil {
		return store, err
	}

	return store, nil
}

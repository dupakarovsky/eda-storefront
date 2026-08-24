package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// DisableoreParticipation defines a struct type that is able to cummunicate
// with the repository and publisher to persist a Store's disable particitpation action
type DisableStoreParticipation struct {
	storeRepo domain.StoreRepository
	publisher *foundationnats.Publisher
	logger    *slog.Logger
}

// NewEnableStoreParticipation creates a new DisableStoreParticipation struct
func NewDisableStoreParticipation(storeRepo domain.StoreRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *DisableStoreParticipation {
	return &DisableStoreParticipation{
		storeRepo: storeRepo,
		publisher: publisher,
		logger:    logger,
	}
}

// Execute will enable store participation action, persist the change to the repository and publish
// the event to NATS
func (uc *DisableStoreParticipation) Execute(ctx context.Context, storeId string) (*domain.Store, error) {

	// locate the store that needs disabling participation
	store, err := uc.storeRepo.FindByID(ctx, storeId)
	if err != nil {
		return nil, fmt.Errorf("finding store: %w", err)
	}

	// store located. disable the participation.
	// ---------------------------------------

	// emit the enable particiatpion event
	event, err := store.DisableParticipation()
	if err != nil {
		return nil, fmt.Errorf("disabling participation: %w", err)
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

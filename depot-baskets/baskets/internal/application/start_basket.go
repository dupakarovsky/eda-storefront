package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// StartBasket defines an application type responsible for perform the orchestration
// across layers. It'll create new Basket and receive it's Event, persist it into the
// database and then publish the event into the jetstream
type StartBasket struct {
	basketRepo domain.BasketRepository
	publisher  *foundationnats.Publisher
	logger     *slog.Logger
}

// NewStartBasket instantiates a new StartBasket USE CASE handler
func NewStartBasket(basketRepo domain.BasketRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *StartBasket {
	return &StartBasket{
		basketRepo: basketRepo,
		publisher:  publisher,
		logger:     logger,
	}
}

// Execute will create a new domain Basket, persist it to the database and publish it's
// event to the stream.
func (uc *StartBasket) Execute(ctx context.Context, id string) (*domain.Basket, error) {

	// Create a new domain Basket type and receive the event
	basket, event, err := domain.NewBasket(id)
	if err != nil {
		return nil, fmt.Errorf("constructing basket: %w", err)
	}

	// Persist the basket into the database
	if err := uc.basketRepo.Save(ctx, basket); err != nil {
		return nil, fmt.Errorf("saving basket: %w", err)
	}

	// publish the event into the the stream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return basket, err
	}

	return basket, nil
}

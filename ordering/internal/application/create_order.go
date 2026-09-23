package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

// CreateOrder defines type use case that can connect to the orders repostiory and
// read/write to the ordering database
type CreateOrder struct {
	orderRepo domain.OrderRepository
	publisher *foundationnats.Publisher
	logger    *slog.Logger
}

// NewCreateOrder instantiates a new NewCreateOrder Use Case type
func NewCreateOrder(orderRepo domain.OrderRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *CreateOrder {
	return &CreateOrder{
		orderRepo: orderRepo,
		publisher: publisher,
		logger:    logger,
	}
}

func (uc *CreateOrder) Execute(ctx context.Context, id string, items []domain.Item) (*domain.Order, error) {

	// create a new Order form the order id and items received from the subscriber
	order, event, err := domain.NewOrder(id, items)
	if err != nil {
		return nil, fmt.Errorf("constructing order: %w", err)
	}

	// use the repository Save method to save the order to the database
	if err := uc.orderRepo.Save(ctx, order); err != nil {
		return nil, fmt.Errorf("saving order: %w", err)
	}

	// publish the order into the stream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {

		//INFO:: If publishDomainEvent fails, the order remains created in the database.
		//Only the event was not pubslihed. This is a bit unusual, but right now it's what
		//we can do until we use a more robusts solution that uses Transaction outbox.
		return order, err
	}

	return order, nil
}

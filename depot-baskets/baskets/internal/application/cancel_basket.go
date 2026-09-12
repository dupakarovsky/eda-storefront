// baskets/internal/application/cancel_basket.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

type CancelBasket struct {
	basketRepo domain.BasketRepository
	publisher  *foundationnats.Publisher
	logger     *slog.Logger
}

func NewCancelBasket(basketRepo domain.BasketRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *CancelBasket {
	return &CancelBasket{
		basketRepo: basketRepo,
		publisher:  publisher,
		logger:     logger,
	}
}

func (uc *CancelBasket) Execute(ctx context.Context, basketID string) (*domain.Basket, error) {

	var event domain.Event
	var basket *domain.Basket

	err := uc.basketRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		basket, err = uc.basketRepo.FindByIDForUpdate(txCtx, basketID)
		if err != nil {
			return fmt.Errorf("finding basket: %w", err)
		}

		event, err = basket.Cancel()
		if err != nil {
			return fmt.Errorf("canceling basket: %w", err)
		}

		if err := uc.basketRepo.Save(txCtx, basket); err != nil {
			return fmt.Errorf("saving basket: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return basket, err
	}

	return basket, nil
}

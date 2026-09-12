// baskets/internal/application/get_basket.go
package application

import (
	"context"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

type GetBasket struct {
	basketRepo domain.BasketRepository
}

func NewGetBasket(basketRepo domain.BasketRepository) *GetBasket {
	return &GetBasket{
		basketRepo: basketRepo,
	}
}

func (uc *GetBasket) Execute(ctx context.Context, id string) (*domain.Basket, error) {

	basket, err := uc.basketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding basket: %w", err)
	}

	return basket, nil
}

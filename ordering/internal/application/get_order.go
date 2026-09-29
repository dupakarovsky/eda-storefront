package application

import (
	"context"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

// GetOrder defines a type that can connect to the order's repository
type GetOrder struct {
	orderRepo domain.OrderRepository
}

// NewGetOrder will instantiate a new GetOrder
func NewGetOrder(orderRepo domain.OrderRepository) *GetOrder {
	return &GetOrder{
		orderRepo: orderRepo,
	}
}

// Execute will simply connect to orders repository and return a orther from the database
func (uc *GetOrder) Execute(ctx context.Context, orderID string) (*domain.Order, error) {
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("finding order: %w", err)
	}

	return order, nil
}

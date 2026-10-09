package domain

import (
	"context"
	"errors"
)

var ErrCustomerNotFound = errors.New("customer not found")

// CustomerRepository defines the behaviour any concrete type should have
// in order to communicate with any database adapter
type CustomerRepository interface {
	//-- write
	Save(ctx context.Context, customer *Customer) error
	FindByIDForUpdate(ctx context.Context, id string) (*Customer, error)
	WithLock(ctx context.Context, fn func(ctx context.Context) error) error
	//-- read
	FindByID(ctx context.Context, id string) (*Customer, error)
}

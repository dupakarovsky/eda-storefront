package domain

import "context"

// TxManager can be used by any concrete type (StoreRepository or ProductRepository) that
// implements a WithTx method. This method will execute a function fn held within
// a transaction lock
type TxManager interface {
	WithTx(ctx context.Context, fn func(context.Context, *StoreRepository, *ProductRepository) error) error
}

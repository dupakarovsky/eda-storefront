// 01-stores/internal/postgres/txcontext.go
package postgres

import "context"

type txKey struct{}

func ctxWithTx(ctx context.Context, tx Executor) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// executorFrom will verify if the context carries a transaction object.
// If it does, it'll return it. Othwerwise it'll return a plain pool of
// connections not initialized in a transaction
func executorFrom(ctx context.Context, db Executor) Executor {
	if tx, ok := ctx.Value(txKey{}).(Executor); ok {
		return tx
	}
	return db
}

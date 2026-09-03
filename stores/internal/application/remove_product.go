package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// RemoveProduct defines a type that is able to remove a Product from the
// repository and emit the corresponding event
type RemoveProduct struct {
	productRepo domain.ProductRepository
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

// NewRemoveProduct instantiates a new RemoveProduct struct
func NewRemoveProduct(productRepo domain.ProductRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *RemoveProduct {
	return &RemoveProduct{
		productRepo: productRepo,
		publisher:   publisher,
		logger:      logger,
	}
}

// Execute will try to locate a product in the database, emit a the Delete stream and
// then remove the product from the repository, Then it'll publish the event
// to the stream
func (uc *RemoveProduct) Execute(ctx context.Context, productID string) error {

	// event and store will capture the values returned by the FindByIDForUpdate and RebrandStore
	// methods
	var event domain.Event
	var product *domain.Product

	// WithLock will start a new transaction where the FindByIDForUpdate
	// method will execute, followed by Delete and Save to presist the
	// changes to the database.
	err := uc.productRepo.WithLock(ctx, func(txCtx context.Context) error {

		var err error

		// locate the product in the database
		product, err = uc.productRepo.FindByIDForUpdate(txCtx, productID)
		if err != nil {
			return fmt.Errorf("finding product: %w", err)
		}

		// product located
		// ----------------

		// call Remove to receive the event
		event, err = product.Remove()
		if err != nil {
			return fmt.Errorf("removing product: %w", err)
		}

		// event emited. remove from the repository
		// ----------------------------------------

		// call Delete to remove the product
		if err := uc.productRepo.Delete(txCtx, productID); err != nil {
			return fmt.Errorf("deleting product: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// product deleted from database.
	// ------------------------------

	//INFO:: If publishDomainEvent fails, the product remains deleted in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	// publish the event
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return err
	}

	return nil
}

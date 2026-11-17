// depot-baskets/depot/internal/postgres/catalog_repository_test.go
package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/depot/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/depot/internal/postgres"
	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationpostgres "github.com/dupakarovsky/eda-storefront/foundation/postgres"
	"github.com/joho/godotenv"
)

func uniqueID(tag string) string {
	return tag + "-" + time.Now().Format("20060102150405.000000")
}

func newCatalogRepo(t *testing.T) *postgres.CatalogRepository {
	t.Helper()

	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load depot/.env: %v", err)
	}

	cfg, err := foundationconfig.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	pool, err := foundationpostgres.NewPool(context.Background(), foundationpostgres.Config{
		Host:            cfg.DB.Host,
		Port:            cfg.DB.Port,
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		Database:        cfg.DB.Database,
		Schema:          cfg.DB.Schema,
		SSLMode:         foundationpostgres.SSLMode(cfg.DB.SSLMode),
		MaxConns:        int32(cfg.DB.MaxConns),
		MinConns:        int32(cfg.DB.MinConns),
		MaxConnLifetime: cfg.DB.MaxConnLifetime,
		MaxConnIdleTime: cfg.DB.MaxConnIdleTime,
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return postgres.NewCatalogRepository(pool)
}

func assertPrice(t *testing.T, repo *postgres.CatalogRepository, id string, want float64) {
	t.Helper()

	product, err := repo.FindProductByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to find product %q: %v", id, err)
	}
	if product.Price != want {
		t.Errorf("expected price %v, got %v", want, product.Price)
	}
}

// depot-baskets/depot/internal/postgres/catalog_repository_test.go (append)

func TestCatalogRepository_AdjustProductPrice_SkipsAlreadyAppliedSequence(t *testing.T) {
	repo := newCatalogRepo(t)
	ctx := context.Background()

	id := uniqueID("adjust-price")
	if err := repo.CreateProduct(ctx, id, "store-1", "Widget", "a test widget", "SKU-ADJ-1", 10.00, 0); err != nil {
		t.Fatalf("failed to create catalog product: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.DeleteProduct(context.Background(), id)
	})

	if err := repo.AdjustProductPrice(ctx, id, 2.50, 100); err != nil {
		t.Fatalf("expected first adjustment to apply, got error: %v", err)
	}
	assertPrice(t, repo, id, 12.50)

	t.Run("redelivery of the same sequence", func(t *testing.T) {
		err := repo.AdjustProductPrice(ctx, id, 2.50, 100)
		if !errors.Is(err, catalog.ErrPriceAdjustmentSkipped) {
			t.Errorf("expected error to wrap catalog.ErrPriceAdjustmentSkipped, got: %v", err)
		}
		assertPrice(t, repo, id, 12.50)
	})

	t.Run("older sequence arriving late", func(t *testing.T) {
		err := repo.AdjustProductPrice(ctx, id, 1.00, 50)
		if !errors.Is(err, catalog.ErrPriceAdjustmentSkipped) {
			t.Errorf("expected error to wrap catalog.ErrPriceAdjustmentSkipped, got: %v", err)
		}
		assertPrice(t, repo, id, 12.50)
	})
}

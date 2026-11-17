package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/postgres"
	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationpostgres "github.com/dupakarovsky/eda-storefront/foundation/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// returns a unique ID for running a test
func uniqueID(tag string) string {
	return tag + "-" + time.Now().Format("20060102150405.000000")
}

// depot-baskets/baskets/internal/postgres/catalog_repository_test.go (changed: pool creation extracted)
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load baskets/.env: %v", err)
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

	return pool
}

func newCatalogRepo(t *testing.T) *postgres.CatalogRepository {
	t.Helper()

	return postgres.NewCatalogRepository(newPool(t))
}

func assertLastAppliedSeq(t *testing.T, id string, want uint64) {
	t.Helper()

	var got uint64
	err := newPool(t).QueryRow(context.Background(),
		`SELECT last_applied_seq FROM catalog_products WHERE id = $1`, id).Scan(&got)
	if err != nil {
		t.Fatalf("failed to read last_applied_seq for %q: %v", id, err)
	}
	if got != want {
		t.Errorf("expected last_applied_seq %d, got %d", want, got)
	}
}

func assertPrice(t *testing.T, repo *postgres.CatalogRepository, id string, want float64) {
	t.Helper()

	// search the database for a product by it's ID
	product, err := repo.FindProductByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to find product %q: %v", id, err)
	}
	// compare it's price with the wanted price
	if product.Price != want {
		t.Errorf("expected price %v, got %v", want, product.Price)
	}
}

//-----------------------------------------------------------------------------

func TestProject_StoresTheStreamSequenceOfEachApplyingEvent(t *testing.T) {
	_, project, id := newProjectedProduct(t, "proj-seq")
	assertLastAppliedSeq(t, id, 10)

	deliver(t, project, id, "ProductPriceIncreased", 11, `{"delta":2,"price":7}`)
	assertLastAppliedSeq(t, id, 11)

	deliver(t, project, id, "ProductPriceDecreased", 9, `{"delta":-1,"price":4}`)
	assertLastAppliedSeq(t, id, 11)
}

func TestCatalogRepository_AdjustProductPrice_SkipsAlreadyAppliedSequence(t *testing.T) {

	// generate a new catalog repo
	repo := newCatalogRepo(t)
	ctx := context.Background()

	// generate a new unique id for the test product
	id := uniqueID("adjust-price")

	// create a new product in the catalog_product table
	if err := repo.CreateProduct(ctx, id, "store-1", "Widget", "a test widget", "SKU-ADJ-1", 10.00, 0); err != nil {
		t.Fatalf("failed to create catalog product: %v", err)
	}
	// when the test finishes, delete the test product
	t.Cleanup(func() {
		_ = repo.DeleteProduct(context.Background(), id)
	})

	// increase the products price by 2.5; should be 12.5 now
	if err := repo.AdjustProductPrice(ctx, id, 2.50, 100); err != nil {
		t.Fatalf("expected first adjustment to apply, got error: %v", err)
	}
	// verify if the price changed in thed atabase
	assertPrice(t, repo, id, 12.50)

	// simulates the jestream redelivering the same sequence. should fail with
	// ErrPriceAdjustmentSkipped
	t.Run("redelivery of the same sequence", func(t *testing.T) {
		err := repo.AdjustProductPrice(ctx, id, 2.50, 100)
		if !errors.Is(err, catalog.ErrPriceAdjustmentSkipped) {
			t.Errorf("expected error to wrap catalog.ErrPriceAdjustmentSkipped, got: %v", err)
		}
		assertPrice(t, repo, id, 12.50)
	})

	// simulates an different message arriving late. The stored sequence number in
	// the database needs to smaller than the one arriving for the update to succeed. Here is not
	// so it shoulr return an ErrPriceAdjustmentSkipped error as well
	t.Run("older sequence arriving late", func(t *testing.T) {
		err := repo.AdjustProductPrice(ctx, id, 1.00, 50)
		if !errors.Is(err, catalog.ErrPriceAdjustmentSkipped) {
			t.Errorf("expected error to wrap catalog.ErrPriceAdjustmentSkipped, got: %v", err)
		}
		assertPrice(t, repo, id, 12.50)
	})
}

func TestCatalogRepository_PriceChanges_ReportUnknownProduct(t *testing.T) {
	repo := newCatalogRepo(t)
	ctx := context.Background()

	t.Run("adjust", func(t *testing.T) {
		err := repo.AdjustProductPrice(ctx, uniqueID("unknown-adjust"), 1.00, 100)
		if !errors.Is(err, catalog.ErrProductNotFound) {
			t.Errorf("expected error to wrap catalog.ErrProductNotFound, got: %v", err)
		}
		if errors.Is(err, catalog.ErrPriceAdjustmentSkipped) {
			t.Errorf("an unknown product must not look like an already-applied event")
		}
	})

	t.Run("set", func(t *testing.T) {
		err := repo.SetProductPrice(ctx, uniqueID("unknown-set"), 1.00, 100)
		if !errors.Is(err, catalog.ErrProductNotFound) {
			t.Errorf("expected error to wrap catalog.ErrProductNotFound, got: %v", err)
		}
		if errors.Is(err, catalog.ErrPriceAdjustmentSkipped) {
			t.Errorf("an unknown product must not look like an already-applied event")
		}
	})
}

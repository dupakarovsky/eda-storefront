package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/postgres"
	"github.com/nats-io/nats.go/jetstream"
)

const productAddedPayload = `{"store_id":"store-1","name":"Widget","description":"a test widget","sku":"SKU-PROJ-1","price":5}`

type fakeMsg struct {
	jetstream.Msg
	data []byte
	seq  uint64
}

func (m *fakeMsg) Data() []byte { return m.data }

func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{
		Sequence:     jetstream.SequencePair{Stream: m.seq, Consumer: m.seq},
		NumDelivered: 1,
	}, nil
}

func newProjector(repo *postgres.CatalogRepository) func(context.Context, jetstream.Msg) error {
	return catalog.Project(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func tryDeliver(project func(context.Context, jetstream.Msg) error, id, event string, seq uint64, data string) error {
	// define a body with a JSON blob
	body := fmt.Sprintf(`{"aggregate_id": %q, "event_name": %q, "occurred_at":"2026-10-15T12:00:00Z", "data":%s}`, id, event, data)

	// call the projector passing in a message with the body and the current sequence
	return project(context.Background(), &fakeMsg{data: []byte(body), seq: seq})
}

// delivery now calls the tryDeliver
func deliver(t *testing.T, project func(context.Context, jetstream.Msg) error, id, event string, seq uint64, data string) {
	t.Helper()

	if err := tryDeliver(project, id, event, seq, data); err != nil {
		t.Fatalf("projecting %s at seq %d failed: %v", event, seq, err)
	}
}

func newProjectedProduct(t *testing.T, tag string) (*postgres.CatalogRepository, func(context.Context, jetstream.Msg) error, string) {
	t.Helper()

	repo := newCatalogRepo(t)
	project := newProjector(repo)
	id := uniqueID(tag)
	t.Cleanup(func() {
		_ = repo.DeleteProduct(context.Background(), id)
	})

	deliver(t, project, id, "ProductAdded", 10, productAddedPayload)
	assertPrice(t, repo, id, 5)

	return repo, project, id
}

func TestProject_ProductAddedReplay_KeepsAdjustedPrice(t *testing.T) {
	repo, project, id := newProjectedProduct(t, "proj-replay")

	deliver(t, project, id, "ProductPriceIncreased", 11, `{"delta":2}`)
	assertPrice(t, repo, id, 7)

	deliver(t, project, id, "ProductAdded", 10, productAddedPayload)
	assertPrice(t, repo, id, 7)
}

func TestProject_PriceEventsOutOfOrder_ConvergeOnLatestPrice(t *testing.T) {
	repo, project, id := newProjectedProduct(t, "proj-reorder")

	deliver(t, project, id, "ProductPriceDecreased", 12, `{"delta":-0.5,"price":6.5}`)
	deliver(t, project, id, "ProductPriceIncreased", 11, `{"delta":2,"price":7}`)

	assertPrice(t, repo, id, 6.5)
}

func TestProject_LegacyPriceEventsWithoutPrice_ApplyDeltas(t *testing.T) {
	repo, project, id := newProjectedProduct(t, "proj-legacy")

	deliver(t, project, id, "ProductPriceIncreased", 11, `{"delta":2}`)
	deliver(t, project, id, "ProductPriceDecreased", 12, `{"delta":-0.5}`)
	assertPrice(t, repo, id, 6.5)

	deliver(t, project, id, "ProductPriceIncreased", 11, `{"delta":2}`)
	assertPrice(t, repo, id, 6.5)
}

func TestProject_PriceEventForUnknownProduct_Fails(t *testing.T) {

	// test the cases for a price message: one with just the delta and another
	// with the absolute price
	cases := []struct {
		name string
		data string
	}{
		{"absolute price", `{"delta":2,"price":7}`},
		{"legacy delta", `{"delta":2}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			// define the new project to post message into the stream
			project := newProjector(newCatalogRepo(t))

			// try executing project with the unkown product
			err := tryDeliver(project, uniqueID("proj-unknown"), "ProductPriceIncreased", 11, tc.data)
			if !errors.Is(err, catalog.ErrProductNotFound) {
				t.Errorf("expected error to wrap catalog.ErrProductNotFound, got: %v", err)
			}
		})
	}
}

func TestProject_PriceEventArrivingBeforeProductAdded_AppliesOnRetry(t *testing.T) {

	// new repository that can comunicate with
	repo := newCatalogRepo(t)

	// new projector th sends messages to the stream
	project := newProjector(repo)

	id := uniqueID("proj-early")

	// removethe project that was added once the test finishes
	t.Cleanup(func() {
		_ = repo.DeleteProduct(context.Background(), id)
	})

	// define the event to be published
	priceEvent := `{"delta":2,"price":7}`

	// try making the delivery to the stream
	if err := tryDeliver(project, id, "ProductPriceIncreased", 21, priceEvent); err == nil {
		t.Fatal("expected the price event to fail while the product is unknown, got nil")
	}

	deliver(t, project, id, "ProductAdded", 20, productAddedPayload)
	assertPrice(t, repo, id, 5)

	deliver(t, project, id, "ProductPriceIncreased", 21, priceEvent)
	assertPrice(t, repo, id, 7)
}

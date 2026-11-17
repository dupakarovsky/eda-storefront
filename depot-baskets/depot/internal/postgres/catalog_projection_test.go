package postgres_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/depot/internal/postgres"
	"github.com/nats-io/nats.go/jetstream"
)

const productAddedPayload = `{"store_id":"store-1","name":"Widget","description":"a test widget","sku":"SKU-PROJ-1","price":5}`

// fakeMsg embeds a jetstream.Msg interface so it'll be allowed to use it
// as long as it implements the methods it needs. Only Data and Metadata are
// implemented here so only these can be used.
type fakeMsg struct {
	jetstream.Msg
	data []byte
	seq  uint64
}

// fakeMsg implementation of Data and Metadata
func (m *fakeMsg) Data() []byte { return m.data }

func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{
		Sequence:     jetstream.SequencePair{Stream: m.seq, Consumer: m.seq},
		NumDelivered: 1,
	}, nil
}

// newProjector fakes a projector function used to store data into the database
func newProjector(repo *postgres.CatalogRepository) func(context.Context, jetstream.Msg) error {
	return catalog.Project(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// deliver fakes a message being delivered to the jetstream
func deliver(t *testing.T, project func(context.Context, jetstream.Msg) error, id, event string, seq uint64, data string) {
	t.Helper()

	body := fmt.Sprintf(`{"aggregate_id":%q,"event_name":%q,"occurred_at":"2026-10-15T12:00:00Z","data":%s}`, id, event, data)
	if err := project(context.Background(), &fakeMsg{data: []byte(body), seq: seq}); err != nil {
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

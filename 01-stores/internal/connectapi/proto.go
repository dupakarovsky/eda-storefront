package connectapi

import (
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
	"github.com/dupakarovsky/eda-storefront/stores/storespb"
)

func toProtoStore(store *domain.Store) *storespb.Store {
	return &storespb.Store{
		Id:            store.ID,
		Name:          store.Name,
		Location:      store.Location,
		Participating: store.Participating,
	}
}

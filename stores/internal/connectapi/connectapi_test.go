package connectapi

import (
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestMapError_PriceWithTooManyDecimals_IsInvalidArgument(t *testing.T) {
	err := mapError(fmt.Errorf("constructing product: %w", domain.ErrProductPriceHasTooManyDecimals))

	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Errorf("expected %v, got %v", connect.CodeInvalidArgument, got)
	}
}

func TestToProtoProduct_CopiesEveryField(t *testing.T) {
	product := domain.ReconstituteProduct("product-1", "store-1", "Widget", "a test widget", "SKU-1", 9.99)

	got := toProtoProduct(product)

	if got.GetId() != "product-1" || got.GetStoreId() != "store-1" || got.GetName() != "Widget" {
		t.Errorf("identity fields not copied: %+v", got)
	}
	if got.GetDescription() != "a test widget" {
		t.Errorf("expected description %q, got %q", "a test widget", got.GetDescription())
	}
	if got.GetSku() != "SKU-1" || got.GetPrice() != 9.99 {
		t.Errorf("sku or price not copied: %+v", got)
	}
}

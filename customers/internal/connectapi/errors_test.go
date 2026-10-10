// customers/internal/connectapi/errors_test.go
package connectapi

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

func codeOf(err error) connect.Code {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return connectErr.Code()
	}
	return connect.CodeUnknown
}

func TestMapError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"not found", domain.ErrCustomerNotFound, connect.CodeNotFound},
		{"blank name", domain.ErrCustomerNameIsBlank, connect.CodeInvalidArgument},
		{"blank sms number", domain.ErrCustomerSmsNumberIsBlank, connect.CodeInvalidArgument},
		{"already disabled", domain.ErrCustomerAlreadyDisabled, connect.CodeFailedPrecondition},
		{"not authorized", domain.ErrCustomerNotAuthorized, connect.CodePermissionDenied},
		{"already enabled", domain.ErrCustomerAlreadyEnabled, connect.CodeFailedPrecondition},
		{"unchanged sms number", domain.ErrCustomerSmsNumberUnchanged, connect.CodeInvalidArgument},
		{"unknown error", errors.New("boom"), connect.CodeInternal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("authorizing customer: %w", fmt.Errorf("finding customer: %w", tc.err))

			if got := codeOf(mapError(wrapped)); got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

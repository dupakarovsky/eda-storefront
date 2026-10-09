package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

// CustomerRegisteredPayload is a type CustomerRegistered will
// be converted to in order to be transported to the jetstream
type CustomerRegisteredPayload struct {
	Name      string `json:"name"`
	SmsNumber string `json:"sms_number"`
}

// CustomerSmsChangedPayload is a type CustomerSmsChanged will
// be converted to in order to be transported to the jetstream
type CustomerSmsChangedPayload struct {
	SmsNumber string `json:"sms_number"`
}

// CustomeEnabledPayload is a type CustomerEnabled will
// be converted to in order to be transported to the jetstream
type CustomerEnabledPayload struct{}

// CustomeDisabledPayload is a type CustomerDisabled will
// be converted to in order to be transported to the jetstream
type CustomerDisabledPayload struct{}

// toPlayload takes in any  Event type, evaluates which
// concrete type it is, and return it's payload form
func toPlayload(event domain.Event) (any, error) {

	switch e := event.(type) {

	case domain.CustomerRegistered:
		return CustomerRegisteredPayload{
			Name:      e.Name,
			SmsNumber: e.SmsNumber,
		}, nil

	case domain.CustomerSmsChanged:
		return CustomerSmsChangedPayload{
			SmsNumber: e.SmsNumber,
		}, nil

	case domain.CustomerEnabled:
		return CustomerEnabledPayload{}, nil

	case domain.CustomerDisabled:
		return CustomerDisabledPayload{}, nil

	default:
		return nil, fmt.Errorf("no payload mapping defined for event type %T", event)
	}
}

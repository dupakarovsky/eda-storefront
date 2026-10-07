package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrCustomerNameIsBlank        = errors.New("the customer name cannot be blank")
	ErrCustomerSmsNumberIsBlank   = errors.New("the customer sms number cannot be blank")
	ErrCustomerSmsNumberUnchanged = errors.New("the sms number is the same as the current one")
	ErrCustomerAlreadyEnabled     = errors.New("the customer is already enabled")
	ErrCustomerAlreadyDisabled    = errors.New("the customer is already disabled")
	ErrCustomerNotAuthorized      = errors.New("the customer is not authorized")
)

// Customer defines a type that  representes a Customer in the system
type Customer struct {
	ID        string
	Name      string
	SmsNumber string
	Enabled   bool
}

// NewCustomer instantiates a new customer
func NewCustomer(id, name, smsNumber string) (*Customer, Event, error) {

	// remove spaces in front and and the end
	name = strings.TrimSpace(name)
	smsNumber = strings.TrimSpace(smsNumber)

	// guard against blank name
	if name == "" {
		return nil, nil, ErrCustomerNameIsBlank
	}

	// guard against blank sms number
	if smsNumber == "" {
		return nil, nil, ErrCustomerSmsNumberIsBlank
	}

	// instantiate a new Customer
	customer := &Customer{
		ID:        id,
		Name:      name,
		SmsNumber: smsNumber,
		Enabled:   true,
	}

	// create the CustomerRegistered event
	event := CustomerRegistered{
		eventBase: eventBase{aggregateID: id, occurredAt: time.Now()},
		Name:      name,
		SmsNumber: smsNumber,
	}

	return customer, event, nil
}

// ChangeSmsNumber updates the Customer number, returning an CustomerSmsChangedEvent
func (c *Customer) ChangeSmsNumber(smsNumber string) (Event, error) {

	// remove leading and trailling spaces
	smsNumber = strings.TrimSpace(smsNumber)

	// guard against blank numbers
	if smsNumber == "" {
		return nil, ErrCustomerSmsNumberIsBlank
	}

	// guard against the same number
	if smsNumber == c.SmsNumber {
		return nil, ErrCustomerSmsNumberUnchanged
	}

	// passed the checks; can be updated
	// ---------------------------------

	// upadate the sms number
	c.SmsNumber = smsNumber

	// construct the CustomerSmsChanged Event type
	event := CustomerSmsChanged{
		eventBase: eventBase{aggregateID: c.ID, occurredAt: time.Now()},
		SmsNumber: smsNumber,
	}

	return event, nil
}

// Enable will enabled a disabled Customer
func (c *Customer) Enable() (Event, error) {

	// verify if the customer is already enabled
	if c.Enabled {
		return nil, ErrCustomerAlreadyEnabled
	}

	// passed; can now be enabled
	// -----------------------------

	// update the property
	c.Enabled = true

	// construct the CustomerEnabled event
	event := CustomerEnabled{
		eventBase: eventBase{aggregateID: c.ID, occurredAt: time.Now()},
	}

	return event, nil
}

// Disable will disabled an enabled  Customer
func (c *Customer) Disable() (Event, error) {

	// verify if the customer is already disabled
	if !c.Enabled {
		return nil, ErrCustomerAlreadyDisabled
	}

	// can be disabled
	//------------------

	// update the properyt
	c.Enabled = false

	// return the CustomerEnabled event
	return CustomerDisabled{
		eventBase: eventBase{aggregateID: c.ID, occurredAt: time.Now()},
	}, nil
}

// Authorize can only will on enabled customers. Emits no Events
func (c *Customer) Authorize() error {
	if !c.Enabled {
		return ErrCustomerNotAuthorized
	}

	return nil
}

// ReconstituteCustomer will reconstruct an Customer type from the primitives
// parameters it's given
func ReconstituteCustomer(id, name, smsNumber string, enabled bool) *Customer {
	return &Customer{
		ID:        id,
		Name:      name,
		SmsNumber: smsNumber,
		Enabled:   enabled,
	}
}

package domain

// OrderStatus defines the status an Order can be in
type OrderStatus string

const (
	OrderCreatedStatus  OrderStatus = "created"
	OrderReadiedStatus  OrderStatus = "readied"
	OrderCanceledStatus OrderStatus = "canceled"
)

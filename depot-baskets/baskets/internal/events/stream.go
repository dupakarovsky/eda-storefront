// baskets/internal/events/stream.go
package events

const (
	BasketsStreamName = "baskets"
	StoresStreamName  = "stores"
)

var BasketsStreamSubjects = []string{
	"baskets.basket.*",
	"baskets.deadletter.>",
}

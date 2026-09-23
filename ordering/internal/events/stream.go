// ordering/internal/events/stream.go
package events

// OrderingStreamName defines the name of the stream the ordering pacakge owns
// BasketsStreamName is the stream "ordering" will consume
const (
	OrderingStreamName = "ordering"
	BasketsStreamName  = "baskets"
)

// OrderingStreamSubjects defines all the subjects under which an event
// can be published
var OrderingStreamSubjects = []string{
	"ordering.order.*",
}

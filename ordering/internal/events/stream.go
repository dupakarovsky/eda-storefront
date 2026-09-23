// ordering/internal/events/stream.go
package events

// OrderingStreamName defines the name of the stream the ordering pacakge owns
// TODO: The baskets, which order will need to published to will be added later
const (
	OrderingStreamName = "ordering"
)

// OrderingStreamSubjects defines all the subjects under which an event
// can be published
var OrderingStreamSubjects = []string{
	"ordering.order.*",
}

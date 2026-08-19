package events

// stream name for the store service
const (
	StreamName = "stores"
)

// StreamSubjects defines the subjects in the "stores" stream that can be subscribed to
var StreamSubjects = []string{
	"stores.store.*",
	"stores.product.*",
}

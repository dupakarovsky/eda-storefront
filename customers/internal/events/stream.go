package events

const (
	StreamName = "customers"
)

// StreamSubjects defines which subjects in the stream this package will publish to
// customer gets no <subject>.deadletter since it doesn't subscribe to (consumes) anything.
var StreamSubjects = []string{
	"customers.customer.*",
}

//	defines all the subjects under which an event can be published.
//* matches exactly one token between dots, while > matches one-or-more trailing tokens.

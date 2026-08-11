package foundationnats

import "fmt"

// Subject dfines string format that consumers will need used
// access events the stream: <service>.<aggregate>.<event>
func Subject(service, aggregate, event string) string {
	return fmt.Sprintf("%s.%s.%s", service, aggregate, event)
}

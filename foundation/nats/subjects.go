package foundationnats

import "fmt"

// Subjects dfines string format that consumers will need used
// access events the stream: <service>.<aggregate>.<event>
func Subjects(service, aggregate, event string) string {
	return fmt.Sprintf("%s.%s.%s.", service, aggregate, event)
}

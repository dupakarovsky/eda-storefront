// depot/internal/events/stream.go
package events

const (
	DepotStreamName  = "depot"
	StoresStreamName = "stores"
)

var DepotStreamSubjects = []string{
	"depot.shopping_list.*",
	"depot.deadletter.*",
}

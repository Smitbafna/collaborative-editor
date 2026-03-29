package websocket

// OperationType represents the type of a document operation.
type OperationType string

const (
	// InsertOperation represents an insert operation.
	InsertOperation OperationType = "insert"
	// DeleteOperation represents a delete operation.
	DeleteOperation OperationType = "delete"
)

// Operation represents a collaborative document operation.
//
// For INSERT operations, Text contains the text to insert at Position.
// For DELETE operations, Length specifies the number of characters to delete starting at Position.
type Operation struct {
	ID       string        `json:"id"`
	ClientID string        `json:"client_id"`
	Type     OperationType `json:"type"`
	Position int           `json:"position"`
	Text     string        `json:"text,omitempty"`
	Length   int           `json:"length,omitempty"`
}
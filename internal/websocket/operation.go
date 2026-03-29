package websocket

import (
	"errors"
	"fmt"
)

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

// Validate checks whether the operation is valid for a document of the given length.
//
// It returns an error describing the first validation failure, or nil if the
// operation is valid. The following rules are enforced:
//
//   - The operation type must be "insert" or "delete".
//   - For INSERT operations:
//     - The text must not be empty.
//     - The position must be in the range [0, contentLength].
//   - For DELETE operations:
//     - The position must be in the range [0, contentLength-1].
//     - The length must be positive.
//     - The position + length must not exceed contentLength.
//
// This method is the server's defence against malformed or malicious client
// input. It must be called before applying any operation received from a
// client.
func (op Operation) Validate(contentLength int) error {
	switch op.Type {
	case InsertOperation:
		if op.Text == "" {
			return errors.New("text must not be empty")
		}
		if op.Position < 0 || op.Position > contentLength {
			return errors.New("position out of bounds")
		}
	case DeleteOperation:
		if op.Position < 0 || op.Position >= contentLength {
			return errors.New("position out of bounds")
		}
		if op.Length <= 0 {
			return errors.New("length must be positive")
		}
		if op.Position+op.Length > contentLength {
			return errors.New("delete range exceeds content length")
		}
	default:
		return fmt.Errorf("invalid operation type: %q", op.Type)
	}
	return nil
}

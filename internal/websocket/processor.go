package websocket

import "fmt"

// ProcessResult holds the outcome of processing an operation.
// It contains the updated document content, the original operation,
// and any error that occurred during processing.
type ProcessResult struct {
	// UpdatedContent is the document content after applying the operation.
	// This is empty if Err is non-nil.
	UpdatedContent string

	// Operation is the operation that was processed.
	// This is always set, even on error, so callers can reference it
	// when constructing error responses.
	Operation Operation

	// Err is non-nil if the operation could not be processed
	// (e.g., validation failure or missing room).
	Err error
}

// OperationProcessor handles the validation and application of operations
// to collaborative documents.
//
// It sits between the WebSocket handler and the Room, providing a clean
// separation of concerns:
//
//	WebSocket Handler
//	       │
//	       ▼
//	Operation Processor
//	       │
//	       ├── Validate
//	       ├── Apply
//	       └── Return result
//
// This abstraction becomes critical when versions and conflict resolution
// are introduced in later milestones.
type OperationProcessor struct {
	roomManager *RoomManager
}

// NewOperationProcessor creates a new OperationProcessor that uses the
// given RoomManager to access document rooms.
func NewOperationProcessor(rm *RoomManager) *OperationProcessor {
	return &OperationProcessor{
		roomManager: rm,
	}
}

// Process validates and applies an operation to the document in the
// specified room.
//
// The flow is:
//  1. Look up the room for the given documentID
//  2. Validate the operation against the current document content
//  3. Apply the operation to the document
//  4. Return the result
//
// If the room is not found, or validation fails, the returned ProcessResult
// will have a non-nil Err field.
func (p *OperationProcessor) Process(documentID string, op Operation) ProcessResult {
	// Step 1: Look up the room
	room, ok := p.roomManager.GetRoom(documentID)
	if !ok {
		return ProcessResult{
			Operation: op,
			Err:       fmt.Errorf("room %s not found", documentID),
		}
	}

	// Step 2: Validate the operation against the current document content
	content := room.GetContent()
	if err := op.Validate(len(content)); err != nil {
		return ProcessResult{
			Operation: op,
			Err:       err,
		}
	}

	// Step 3: Apply the operation to the document
	updatedContent := room.ApplyOperation(op)

	// Step 4: Return the result
	return ProcessResult{
		UpdatedContent: updatedContent,
		Operation:      op,
	}
}
package websocket

import "fmt"

var (
	// ErrStaleOperation is returned when a client submits an operation
	// based on an outdated document version.
	ErrStaleOperation = fmt.Errorf("STALE_OPERATION")

	// ErrInvalidBaseVersion is returned when a client submits an operation
	// with a base version that is ahead of the server's current version.
	// This indicates the client's state is inconsistent with the server.
	ErrInvalidBaseVersion = fmt.Errorf("INVALID_BASE_VERSION")
)

// ProcessResult holds the outcome of processing an operation.
// It contains the updated document content, the original operation,
// the new document version, and any error that occurred during processing.
type ProcessResult struct {
	// UpdatedContent is the document content after applying the operation.
	// This is empty if Err is non-nil or if the operation was a duplicate.
	UpdatedContent string

	// Operation is the operation that was processed.
	// This is always set, even on error, so callers can reference it
	// when constructing error responses.
	Operation Operation

	// Version is the document version after the operation was applied.
	// This is the server's version, which clients should use to update
	// their local base version.
	Version int64

	// Err is non-nil if the operation could not be processed
	// (e.g., validation failure, version mismatch, or missing room).
	Err error

	// IsDuplicate is true if the operation ID had already been processed.
	// Callers should not broadcast the operation again when this is true.
	IsDuplicate bool
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
//  2. Check if the operation has already been processed (idempotency)
//  3. Validate the operation against the current document content
//  4. Check the client's BaseVersion against the server's current version
//  5. Apply the operation to the document
//  6. Return the result (including the new version)
//
// Version Flow:
//
// The version check ensures that the client's view of the document is
// up-to-date before applying the operation. The client sends its
// BaseVersion, and the server compares it with the current room Version:
//
//	Client BaseVersion == Server Version  →  operation can be applied directly
//	Client BaseVersion != Server Version  →  version mismatch, operation rejected
//
// When BaseVersion is 0 (unset), the version check is skipped for backward
// compatibility with clients that do not yet support versioning.
//
// If the room is not found, or validation fails, or the version check
// fails, the returned ProcessResult will have a non-nil Err field.
//
// If the operation has already been processed, ProcessResult.UpdatedContent
// will be empty and ProcessResult.Err will be nil. Callers should check both
// fields (or use the dedicated ProcessResult helpers) to detect duplicates
// without treating them as errors.
func (p *OperationProcessor) Process(documentID string, op Operation) ProcessResult {
	// Step 1: Look up the room
	room, ok := p.roomManager.GetRoom(documentID)
	if !ok {
		return ProcessResult{
			Operation: op,
			Err:       fmt.Errorf("room %s not found", documentID),
		}
	}

	// Step 2: Process the operation atomically under the room's write lock.
	// ProcessOperation handles idempotency, base-version check, validation,
	// application, version increment, and history storage — all under a single
	// lock acquisition to prevent the race described in Step 17.
	result := room.ProcessOperation(op)

	if result.IsDuplicate {
		return ProcessResult{
			Operation:   op,
			IsDuplicate: true,
		}
	}

	if result.Err != nil {
		// Map stale/invalid base version errors to the processor's sentinel errors
		if result.Err == ErrStaleOperation {
			return ProcessResult{
				Operation: op,
				Err:       ErrStaleOperation,
			}
		}
		if result.Err == ErrInvalidBaseVersion {
			return ProcessResult{
				Operation: op,
				Err:       ErrInvalidBaseVersion,
			}
		}
		return ProcessResult{
			Operation: op,
			Err:       result.Err,
		}
	}

	// Step 3: Return the result (including the new version)
	return ProcessResult{
		UpdatedContent: result.Content,
		Operation:      op,
		Version:        int64(result.Version),
	}
}

package websocket

// Message envelope types sent over WebSocket.
//
// Each message is a JSON object with a "type" field that identifies
// the kind of message, allowing receivers to route messages correctly.
//
// Four message types are defined:
//
//  1. Operation — a collaborative operation applied by another client:
//     {"type": "operation", "operation": {...}, "version": 6}
//
//  2. DocumentSnapshot — the current document content and version, sent
//     when a client first joins a room:
//     {"type": "document_snapshot", "content": "...", "version": 5}
//
//  3. Error — an error response for a rejected operation:
//     {"type": "error", "message": "..."}
//
//  4. SyncRequired — sent when a client's operation is rejected due to
//     a stale base version, instructing the client to synchronize:
//     {"type": "sync_required", "current_version": 5, "operations": [...]}

// OperationMessage wraps an Operation in the message envelope.
//
// This message is broadcast to all clients in a room (except the sender)
// after an operation has been successfully applied to the document.
// Receiving clients apply the operation locally to stay in sync.
//
// The Version field indicates the document version after the operation
// was applied. Clients should update their local base version to this
// value so that subsequent operations they send will have the correct
// BaseVersion.
type OperationMessage struct {
	Type      string    `json:"type"`
	Operation Operation `json:"operation"`
	Version   int64     `json:"version"`
}

// NewOperationMessage creates an OperationMessage for the given operation
// and the document version after the operation was applied.
func NewOperationMessage(op Operation, version int64) OperationMessage {
	return OperationMessage{
		Type:      "operation",
		Operation: op,
		Version:   version,
	}
}

// DocumentSnapshotMessage contains the full document content and version.
//
// This message is sent to a client when they first join a room, so they
// know the current document state before receiving any operations.
//
// The Version field tells the client what document version its local copy
// represents. This is critical: without the version, the client cannot
// correctly set its BaseVersion for subsequent operations, and the server's
// version check would reject every operation the client sends.
type DocumentSnapshotMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Version int64  `json:"version"`
}

// NewDocumentSnapshot creates a DocumentSnapshotMessage for the given content
// and version.
func NewDocumentSnapshot(content string, version int64) DocumentSnapshotMessage {
	return DocumentSnapshotMessage{
		Type:    "document_snapshot",
		Content: content,
		Version: version,
	}
}

// ErrorMessage describes why an operation was rejected.
//
// This message is sent back to the client that submitted the rejected
// operation. It contains a human-readable error description.
type ErrorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// NewErrorMessage creates an ErrorMessage with the given text.
func NewErrorMessage(message string) ErrorMessage {
	return ErrorMessage{
		Type:    "error",
		Message: message,
	}
}

// SyncRequiredMessage is sent to a client when its operation is rejected
// due to a stale base version. It instructs the client to synchronize
// by applying the missing operations.
//
// The message contains:
//   - CurrentVersion: the server's current document version
//   - Operations: the operations the client needs to apply to catch up
type SyncRequiredMessage struct {
	Type           string        `json:"type"`
	CurrentVersion int64         `json:"current_version"`
	Operations     []SyncOperation `json:"operations"`
}

// SyncOperation pairs a version number with the operation that created it.
type SyncOperation struct {
	Version   int64      `json:"version"`
	Operation Operation `json:"operation"`
}

// RequestMissingOperationsMessage is sent from client to server to request
// operations that the client missed due to a version gap.
type RequestMissingOperationsMessage struct {
	Type      string `json:"type"`
	AfterVersion int64 `json:"after_version"`
}

// NewRequestMissingOperationsMessage creates a message requesting operations
// after the specified version.
func NewRequestMissingOperationsMessage(afterVersion int64) RequestMissingOperationsMessage {
	return RequestMissingOperationsMessage{
		Type:        "request_missing_operations",
		AfterVersion: afterVersion,
	}
}

// NewSyncRequiredMessage creates a SyncRequiredMessage for the given
// current version and list of missing operations.
func NewSyncRequiredMessage(currentVersion int64, operations []SyncOperation) SyncRequiredMessage {
	return SyncRequiredMessage{
		Type:           "sync_required",
		CurrentVersion: currentVersion,
		Operations:     operations,
	}
}

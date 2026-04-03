package websocket

import (
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Step 5: Operation History Tests
// ---------------------------------------------------------------------------

// TestRoomHistoryEmptyOnNewRoom verifies that a newly created room has an
// empty (but non-nil) operation history.
func TestRoomHistoryEmptyOnNewRoom(t *testing.T) {
	room := NewRoom("history-test-room")

	history := room.GetHistory()

	if history == nil {
		t.Fatal("expected non-nil history slice for new room")
	}
	if len(history) != 0 {
		t.Errorf("expected empty history for new room, got %d entries", len(history))
	}
}

// TestRoomHistoryRecordsAppliedOperations verifies that each successfully
// applied operation is recorded in the history with the correct version
// number.
//
// Scenario:
//   - Room starts empty (version 0)
//   - INSERT "Hello" at position 0 → version 1
//   - INSERT " World" at position 5 → version 2
//
// History should contain 2 entries with versions 1 and 2.
func TestRoomHistoryRecordsAppliedOperations(t *testing.T) {
	room := NewRoom("history-record-test")

	op1 := Operation{ID: "op-001", Type: InsertOperation, Position: 0, Text: "Hello"}
	op2 := Operation{ID: "op-002", Type: InsertOperation, Position: 5, Text: " World"}

	_, _ = room.ApplyOperation(op1)
	_, _ = room.ApplyOperation(op2)

	history := room.GetHistory()

	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}

	// Verify version numbers
	if history[0].Version != 1 {
		t.Errorf("expected first entry version 1, got %d", history[0].Version)
	}
	if history[1].Version != 2 {
		t.Errorf("expected second entry version 2, got %d", history[1].Version)
	}

	// Verify operations are recorded correctly
	if history[0].Operation.Type != InsertOperation {
		t.Errorf("expected first entry operation type 'insert', got '%v'", history[0].Operation.Type)
	}
	if history[0].Operation.Text != "Hello" {
		t.Errorf("expected first entry text 'Hello', got '%s'", history[0].Operation.Text)
	}
	if history[1].Operation.Text != " World" {
		t.Errorf("expected second entry text ' World', got '%s'", history[1].Operation.Text)
	}
}

// TestRoomHistoryDoesNotRecordDuplicates verifies that duplicate operations
// (same ID) are not recorded in the history.
func TestRoomHistoryDoesNotRecordDuplicates(t *testing.T) {
	room := NewRoom("history-dup-test")

	op := Operation{ID: "op-dup", Type: InsertOperation, Position: 0, Text: "Hello"}

	// First application — should be recorded
	_, _ = room.ApplyOperation(op)

	// Second application (duplicate) — should NOT be recorded
	_, _ = room.ApplyOperation(op)

	history := room.GetHistory()

	if len(history) != 1 {
		t.Errorf("expected 1 history entry (duplicate should not be recorded), got %d", len(history))
	}
}

// TestRoomHistoryDoesNotRecordInvalidOperations verifies that operations
// which do not change the document content (invalid operations) are not
// recorded in the history.
func TestRoomHistoryDoesNotRecordInvalidOperations(t *testing.T) {
	room := NewRoom("history-invalid-test")
	room.SetContent("Hello")

	// Invalid position — should not change content, should not be recorded
	op := Operation{ID: "op-invalid", Type: InsertOperation, Position: 100, Text: "X"}
	_, _ = room.ApplyOperation(op)

	history := room.GetHistory()

	if len(history) != 0 {
		t.Errorf("expected 0 history entries for invalid operation, got %d", len(history))
	}
}

// TestRoomHistoryRecordsEmptyIDOperations verifies that operations without
// an ID are still recorded in the history if they change the document content.
// (Empty ID operations bypass idempotency but still modify the document.)
func TestRoomHistoryRecordsEmptyIDOperations(t *testing.T) {
	room := NewRoom("history-empty-id-test")

	op := Operation{Type: InsertOperation, Position: 0, Text: "Hello"}

	_, _ = room.ApplyOperation(op)

	history := room.GetHistory()

	if len(history) != 1 {
		t.Fatalf("expected 1 history entry for empty-ID operation, got %d", len(history))
	}
	if history[0].Operation.Text != "Hello" {
		t.Errorf("expected recorded text 'Hello', got '%s'", history[0].Operation.Text)
	}
}

// TestRoomHistoryGetHistoryReturnsCopy verifies that GetHistory returns a
// copy of the internal history slice, so modifications to the returned slice
// do not affect the room's internal state.
func TestRoomHistoryGetHistoryReturnsCopy(t *testing.T) {
	room := NewRoom("history-copy-test")

	op := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}
	_, _ = room.ApplyOperation(op)

	history := room.GetHistory()

	// Modify the returned slice
	history[0].Operation.Text = "MODIFIED"
	history = append(history, VersionedOperation{Version: 999, Operation: Operation{ID: "fake"}})

	// Verify the room's internal history is unchanged
	internalHistory := room.GetHistory()

	if len(internalHistory) != 1 {
		t.Fatalf("expected 1 internal history entry, got %d", len(internalHistory))
	}
	if internalHistory[0].Operation.Text != "Hello" {
		t.Errorf("expected internal history text 'Hello' (unmodified), got '%s'", internalHistory[0].Operation.Text)
	}
}

// TestRoomHistoryMultipleOperations verifies the full example from the task
// specification:
//
//	Room: document-123
//	Current Version: 3
//
//	History:
//	├── Version 1
//	│   └── INSERT "Hello"
//	├── Version 2
//	│   └── INSERT " World"
//	└── Version 3
//	    └── DELETE "World"
func TestRoomHistoryMultipleOperations(t *testing.T) {
	room := NewRoom("document-123")

	// Version 1: INSERT "Hello"
	op1 := Operation{ID: "op-001", Type: InsertOperation, Position: 0, Text: "Hello"}
	_, _ = room.ApplyOperation(op1)

	// Version 2: INSERT " World"
	op2 := Operation{ID: "op-002", Type: InsertOperation, Position: 5, Text: " World"}
	_, _ = room.ApplyOperation(op2)

	// Version 3: DELETE "World" (6 characters at position 6)
	op3 := Operation{ID: "op-003", Type: DeleteOperation, Position: 6, Length: 5}
	_, _ = room.ApplyOperation(op3)

	// Verify final state
	if room.GetContent() != "Hello " {
		t.Errorf("expected final content 'Hello ', got '%s'", room.GetContent())
	}
	if room.GetVersion() != 3 {
		t.Errorf("expected final version 3, got %d", room.GetVersion())
	}

	// Verify history
	history := room.GetHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}

	// Version 1: INSERT "Hello"
	if history[0].Version != 1 {
		t.Errorf("expected entry 0 version 1, got %d", history[0].Version)
	}
	if history[0].Operation.Type != InsertOperation {
		t.Errorf("expected entry 0 type 'insert', got '%v'", history[0].Operation.Type)
	}
	if history[0].Operation.Text != "Hello" {
		t.Errorf("expected entry 0 text 'Hello', got '%s'", history[0].Operation.Text)
	}

	// Version 2: INSERT " World"
	if history[1].Version != 2 {
		t.Errorf("expected entry 1 version 2, got %d", history[1].Version)
	}
	if history[1].Operation.Type != InsertOperation {
		t.Errorf("expected entry 1 type 'insert', got '%v'", history[1].Operation.Type)
	}
	if history[1].Operation.Text != " World" {
		t.Errorf("expected entry 1 text ' World', got '%s'", history[1].Operation.Text)
	}

	// Version 3: DELETE "World"
	if history[2].Version != 3 {
		t.Errorf("expected entry 2 version 3, got %d", history[2].Version)
	}
	if history[2].Operation.Type != DeleteOperation {
		t.Errorf("expected entry 2 type 'delete', got '%v'", history[2].Operation.Type)
	}
	if history[2].Operation.Length != 5 {
		t.Errorf("expected entry 2 length 5, got %d", history[2].Operation.Length)
	}
}

// TestRoomHistoryVersionSequence verifies that version numbers in the history
// are sequential (1, 2, 3, ...) with no gaps.
func TestRoomHistoryVersionSequence(t *testing.T) {
	room := NewRoom("history-sequence-test")

	// Apply 5 operations
	for i := 0; i < 5; i++ {
		op := Operation{
			ID:       "op-seq-" + string(rune('a'+i)),
			Type:     InsertOperation,
			Position: 0,
			Text:     "x",
		}
		_, _ = room.ApplyOperation(op)
	}

	history := room.GetHistory()

	if len(history) != 5 {
		t.Fatalf("expected 5 history entries, got %d", len(history))
	}

	for i, entry := range history {
		expectedVersion := i + 1
		if entry.Version != expectedVersion {
			t.Errorf("entry %d: expected version %d, got %d", i, expectedVersion, entry.Version)
		}
	}
}

// TestRoomHistoryInsertAndDeleteMix verifies that a mix of INSERT and DELETE
// operations are all correctly recorded in the history.
func TestRoomHistoryInsertAndDeleteMix(t *testing.T) {
	room := NewRoom("history-mix-test")

	// INSERT "Hello World" → version 1
	op1 := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello World"}
	_, _ = room.ApplyOperation(op1)

	// DELETE " World" (6 chars at position 5) → version 2
	op2 := Operation{ID: "op-2", Type: DeleteOperation, Position: 5, Length: 6}
	_, _ = room.ApplyOperation(op2)

	// INSERT " Beautiful" at position 5 → version 3
	op3 := Operation{ID: "op-3", Type: InsertOperation, Position: 5, Text: " Beautiful"}
	_, _ = room.ApplyOperation(op3)

	history := room.GetHistory()

	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}

	// Verify the types alternate correctly
	expectedTypes := []OperationType{InsertOperation, DeleteOperation, InsertOperation}
	for i, entry := range history {
		if entry.Operation.Type != expectedTypes[i] {
			t.Errorf("entry %d: expected type '%v', got '%v'", i, expectedTypes[i], entry.Operation.Type)
		}
	}

	// Verify final content
	if room.GetContent() != "Hello Beautiful" {
		t.Errorf("expected final content 'Hello Beautiful', got '%s'", room.GetContent())
	}
}

// TestRoomHistoryWithProcessor verifies that the OperationProcessor correctly
// records operations in the room's history when processing operations through
// the full pipeline.
func TestRoomHistoryWithProcessor(t *testing.T) {
	rm := NewRoomManager()
	room := rm.CreateRoom("history-processor-test")

	processor := NewOperationProcessor(rm)

	op1 := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello", BaseVersion: 0}
	op2 := Operation{ID: "op-2", Type: InsertOperation, Position: 5, Text: " World", BaseVersion: 1}

	result1 := processor.Process("history-processor-test", op1)
	if result1.Err != nil {
		t.Fatalf("op 1: expected no error, got: %v", result1.Err)
	}

	result2 := processor.Process("history-processor-test", op2)
	if result2.Err != nil {
		t.Fatalf("op 2: expected no error, got: %v", result2.Err)
	}

	history := room.GetHistory()

	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}

	if history[0].Version != 1 {
		t.Errorf("expected first entry version 1, got %d", history[0].Version)
	}
	if history[1].Version != 2 {
		t.Errorf("expected second entry version 2, got %d", history[1].Version)
	}

	if history[0].Operation.Text != "Hello" {
		t.Errorf("expected first entry text 'Hello', got '%s'", history[0].Operation.Text)
	}
	if history[1].Operation.Text != " World" {
		t.Errorf("expected second entry text ' World', got '%s'", history[1].Operation.Text)
	}
}

// TestRoomHistoryDuplicateAfterVersionCheck verifies that when a duplicate
// operation is detected (even after a version mismatch), it is not recorded
// in the history.
func TestRoomHistoryDuplicateAfterVersionCheck(t *testing.T) {
	rm := NewRoomManager()
	room := rm.CreateRoom("history-dup-version-test")
	room.Content = "Hello World"
	room.Version = 5

	processor := NewOperationProcessor(rm)

	op := Operation{
		ID:          "op-dup-version",
		Type:        InsertOperation,
		Position:    11,
		Text:        "!",
		BaseVersion: 5,
	}

	// First application — should succeed and be recorded
	result1 := processor.Process("history-dup-version-test", op)
	if result1.Err != nil {
		t.Fatalf("first apply: expected no error, got: %v", result1.Err)
	}

	// Second application (duplicate) — should be detected as duplicate
	// even though version check would fail (BaseVersion 5 != Server 6)
	result2 := processor.Process("history-dup-version-test", op)
	if result2.Err != nil {
		t.Fatalf("duplicate apply: expected no error, got: %v", result2.Err)
	}
	if !result2.IsDuplicate {
		t.Fatal("expected IsDuplicate to be true for duplicate operation")
	}

	history := room.GetHistory()

	if len(history) != 1 {
		t.Errorf("expected 1 history entry (duplicate should not be recorded), got %d", len(history))
	}
}

// ---------------------------------------------------------------------------
// Step 6: Store the Version on Every Operation Tests
// ---------------------------------------------------------------------------

// TestStep6StoreVersionOnEveryOperation tests the exact scenario from the
// task specification:
//
//	Current version: 5
//	Operation arrives with BaseVersion: 5
//	Server applies it → new version becomes 6
//	History stores: VersionedOperation { Version: 6, Operation: operation }
//
// The operation { id: "op-123", base_version: 5 } becomes:
//
//	History:
//	Version 6: { id: "op-123", base_version: 5 }
//
// The operation is associated with the version it *created* (6), not the
// version it was *based on* (5).
func TestStep6StoreVersionOnEveryOperation(t *testing.T) {
	room := NewRoom("step6-test")
	room.Content = "Hello World"
	room.Version = 5

	op := Operation{
		ID:          "op-123",
		Type:        InsertOperation,
		Position:    11,
		Text:        "!",
		BaseVersion: 5,
	}

	// Verify the initial state matches the task
	if room.Version != 5 {
		t.Fatalf("expected initial server version 5, got %d", room.Version)
	}
	if room.Content != "Hello World" {
		t.Fatalf("expected initial content 'Hello World', got '%s'", room.Content)
	}
	if op.BaseVersion != 5 {
		t.Fatalf("expected operation base_version 5, got %d", op.BaseVersion)
	}

	// Apply the operation directly (simulating the version check passing)
	updatedContent, newVersion := room.ApplyOperation(op)

	// Verify the new version is 6 (not 5)
	if newVersion != 6 {
		t.Errorf("expected new version 6, got %d", newVersion)
	}
	if room.GetVersion() != 6 {
		t.Errorf("expected room version 6, got %d", room.GetVersion())
	}

	// Verify the content was updated
	if updatedContent != "Hello World!" {
		t.Errorf("expected content 'Hello World!', got '%s'", updatedContent)
	}
	if room.GetContent() != "Hello World!" {
		t.Errorf("expected room content 'Hello World!', got '%s'", room.GetContent())
	}

	// Verify the operation is stored in history at version 6
	history := room.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	// The history entry should have Version 6 (the version it created)
	// NOT Version 5 (the base_version)
	if history[0].Version != 6 {
		t.Errorf("expected history entry version 6, got %d", history[0].Version)
	}

	// The operation stored in history should have base_version 5
	if history[0].Operation.ID != "op-123" {
		t.Errorf("expected history entry operation ID 'op-123', got '%s'", history[0].Operation.ID)
	}
	if history[0].Operation.BaseVersion != 5 {
		t.Errorf("expected history entry operation base_version 5, got %d", history[0].Operation.BaseVersion)
	}
}

// TestStep6StoreVersionOnEveryOperationThroughProcessor tests the scenario
// through the OperationProcessor, which is the production code path.
//
// This verifies that when Client BaseVersion == Server Version, the processor
// applies the operation directly, increments the version, and stores the
// operation in history with the version it created.
func TestStep6StoreVersionOnEveryOperationThroughProcessor(t *testing.T) {
	rm := NewRoomManager()
	room := rm.CreateRoom("step6-processor-test")
	room.Content = "Hello World"
	room.Version = 5

	processor := NewOperationProcessor(rm)

	op := Operation{
		ID:          "op-123",
		Type:        InsertOperation,
		Position:    11,
		Text:        "!",
		BaseVersion: 5,
	}

	// Process the operation
	result := processor.Process("step6-processor-test", op)

	if result.Err != nil {
		t.Fatalf("expected no error, got: %v", result.Err)
	}
	if result.Version != 6 {
		t.Errorf("expected result version 6, got %d", result.Version)
	}
	if result.UpdatedContent != "Hello World!" {
		t.Errorf("expected content 'Hello World!', got '%s'", result.UpdatedContent)
	}

	// Verify the operation is stored in history at version 6
	history := room.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Version != 6 {
		t.Errorf("expected history entry version 6, got %d", history[0].Version)
	}
	if history[0].Operation.ID != "op-123" {
		t.Errorf("expected history entry operation ID 'op-123', got '%s'", history[0].Operation.ID)
	}
	if history[0].Operation.BaseVersion != 5 {
		t.Errorf("expected history entry operation base_version 5, got %d", history[0].Operation.BaseVersion)
	}

	// Verify GetHistoryByVersion returns the correct entry for version 6
	entry, ok := room.GetHistoryByVersion(6)
	if !ok {
		t.Fatal("expected to find history entry at version 6")
	}
	if entry.Operation.ID != "op-123" {
		t.Errorf("expected operation ID 'op-123', got '%s'", entry.Operation.ID)
	}
	if entry.Operation.BaseVersion != 5 {
		t.Errorf("expected base_version 5, got %d", entry.Operation.BaseVersion)
	}

	// Verify GetHistoryByVersion returns false for version 5 (the base_version,
	// not the version created by an operation)
	_, ok = room.GetHistoryByVersion(5)
	if ok {
		t.Error("expected no history entry at version 5 (base_version, not created version)")
	}
}

// TestStep6StoreVersionOnEveryOperationMultiple verifies that each operation
// is stored with the version it created, not the version it was based on.
//
// Scenario:
//   - Version 5, content "Hello World"
//   - Operation 1: base_version 5 → creates version 6
//   - Operation 2: base_version 6 → creates version 7
//
// History should have:
//   - Version 6: { id: "op-1", base_version: 5 }
//   - Version 7: { id: "op-2", base_version: 6 }
func TestStep6StoreVersionOnEveryOperationMultiple(t *testing.T) {
	room := NewRoom("step6-multiple-test")
	room.Content = "Hello World"
	room.Version = 5

	// Operation 1: base_version 5 → creates version 6
	op1 := Operation{
		ID:          "op-1",
		Type:        InsertOperation,
		Position:    11,
		Text:        "!",
		BaseVersion: 5,
	}
	_, v1 := room.ApplyOperation(op1)
	if v1 != 6 {
		t.Errorf("expected version 6 after op1, got %d", v1)
	}

	// Operation 2: base_version 6 → creates version 7
	op2 := Operation{
		ID:          "op-2",
		Type:        InsertOperation,
		Position:    12,
		Text:        "!",
		BaseVersion: 6,
	}
	_, v2 := room.ApplyOperation(op2)
	if v2 != 7 {
		t.Errorf("expected version 7 after op2, got %d", v2)
	}

	// Verify history
	history := room.GetHistory()
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}

	// Each entry should have the version it created (not the base_version)
	if history[0].Version != 6 {
		t.Errorf("expected entry 0 version 6, got %d", history[0].Version)
	}
	if history[0].Operation.BaseVersion != 5 {
		t.Errorf("expected entry 0 base_version 5, got %d", history[0].Operation.BaseVersion)
	}

	if history[1].Version != 7 {
		t.Errorf("expected entry 1 version 7, got %d", history[1].Version)
	}
	if history[1].Operation.BaseVersion != 6 {
		t.Errorf("expected entry 1 base_version 6, got %d", history[1].Operation.BaseVersion)
	}

	// Verify GetHistoryByVersion for each
	entry1, ok := room.GetHistoryByVersion(6)
	if !ok {
		t.Fatal("expected to find history entry at version 6")
	}
	if entry1.Operation.ID != "op-1" {
		t.Errorf("expected operation ID 'op-1' at version 6, got '%s'", entry1.Operation.ID)
	}

	entry2, ok := room.GetHistoryByVersion(7)
	if !ok {
		t.Fatal("expected to find history entry at version 7")
	}
	if entry2.Operation.ID != "op-2" {
		t.Errorf("expected operation ID 'op-2' at version 7, got '%s'", entry2.Operation.ID)
	}
}

// TestStep6StoreVersionOnEveryOperationDelete verifies that DELETE operations
// are also stored with the version they created.
func TestStep6StoreVersionOnEveryOperationDelete(t *testing.T) {
	room := NewRoom("step6-delete-test")
	room.Content = "Hello World"
	room.Version = 5

	op := Operation{
		ID:          "op-delete",
		Type:        DeleteOperation,
		Position:    5,
		Length:      1,
		BaseVersion: 5,
	}

	updatedContent, newVersion := room.ApplyOperation(op)

	if newVersion != 6 {
		t.Errorf("expected new version 6, got %d", newVersion)
	}
	if updatedContent != "HelloWorld" {
		t.Errorf("expected content 'HelloWorld', got '%s'", updatedContent)
	}

	history := room.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Version != 6 {
		t.Errorf("expected history entry version 6, got %d", history[0].Version)
	}
	if history[0].Operation.BaseVersion != 5 {
		t.Errorf("expected history entry operation base_version 5, got %d", history[0].Operation.BaseVersion)
	}
	if history[0].Operation.Type != DeleteOperation {
		t.Errorf("expected history entry operation type 'delete', got '%v'", history[0].Operation.Type)
	}
}

// TestGetHistoryByVersion verifies the GetHistoryByVersion method returns
// the correct VersionedOperation for a given version.
func TestGetHistoryByVersion(t *testing.T) {
	room := NewRoom("history-by-version-test")

	// Apply 3 operations
	op1 := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}
	op2 := Operation{ID: "op-2", Type: InsertOperation, Position: 5, Text: " World"}
	op3 := Operation{ID: "op-3", Type: DeleteOperation, Position: 5, Length: 6}

	_, _ = room.ApplyOperation(op1)
	_, _ = room.ApplyOperation(op2)
	_, _ = room.ApplyOperation(op3)

	// Look up each version
	for version, expectedOpID := range map[int]string{
		1: "op-1",
		2: "op-2",
		3: "op-3",
	} {
		entry, ok := room.GetHistoryByVersion(version)
		if !ok {
			t.Errorf("expected to find history entry at version %d", version)
			continue
		}
		if entry.Version != version {
			t.Errorf("expected entry version %d, got %d", version, entry.Version)
		}
		if entry.Operation.ID != expectedOpID {
			t.Errorf("expected operation ID '%s' at version %d, got '%s'", expectedOpID, version, entry.Operation.ID)
		}
	}
}

// TestGetHistoryByVersionNotFound verifies that GetHistoryByVersion returns
// false for a version that doesn't exist in the history.
func TestGetHistoryByVersionNotFound(t *testing.T) {
	room := NewRoom("history-by-version-not-found-test")

	op := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}
	_, _ = room.ApplyOperation(op)

	// Version 1 exists
	_, ok := room.GetHistoryByVersion(1)
	if !ok {
		t.Error("expected to find history entry at version 1")
	}

	// Version 0 doesn't exist (versions start at 1)
	_, ok = room.GetHistoryByVersion(0)
	if ok {
		t.Error("expected no history entry at version 0")
	}

	// Version 2 doesn't exist (only 1 operation applied)
	_, ok = room.GetHistoryByVersion(2)
	if ok {
		t.Error("expected no history entry at version 2")
	}

	// Version 99 doesn't exist
	_, ok = room.GetHistoryByVersion(99)
	if ok {
		t.Error("expected no history entry at version 99")
	}
}

// TestGetHistoryByVersionEmptyHistory verifies that GetHistoryByVersion
// returns false when the history is empty.
func TestGetHistoryByVersionEmptyHistory(t *testing.T) {
	room := NewRoom("history-by-version-empty-test")

	_, ok := room.GetHistoryByVersion(1)
	if ok {
		t.Error("expected no history entry in empty history")
	}
}

// TestStep6StoreVersionOnEveryOperationBackwardCompatibility verifies that
// operations with BaseVersion 0 (unset) are also stored with the correct
// version in history.
func TestStep6StoreVersionOnEveryOperationBackwardCompatibility(t *testing.T) {
	room := NewRoom("step6-backward-compat-test")
	room.Content = ""
	room.Version = 0

	op := Operation{
		ID:          "op-no-version",
		Type:        InsertOperation,
		Position:    0,
		Text:        "Hello",
		BaseVersion: 0, // Unset — backward compatibility mode
	}

	updatedContent, newVersion := room.ApplyOperation(op)

	if newVersion != 1 {
		t.Errorf("expected new version 1, got %d", newVersion)
	}
	if updatedContent != "Hello" {
		t.Errorf("expected content 'Hello', got '%s'", updatedContent)
	}

	history := room.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Version != 1 {
		t.Errorf("expected history entry version 1, got %d", history[0].Version)
	}
	if history[0].Operation.BaseVersion != 0 {
		t.Errorf("expected history entry operation base_version 0, got %d", history[0].Operation.BaseVersion)
	}

	// Verify GetHistoryByVersion(1) returns the entry
	entry, ok := room.GetHistoryByVersion(1)
	if !ok {
		t.Fatal("expected to find history entry at version 1")
	}
	if entry.Operation.ID != "op-no-version" {
		t.Errorf("expected operation ID 'op-no-version', got '%s'", entry.Operation.ID)
	}
}

// ---------------------------------------------------------------------------
// Step 11: History Queries Tests
// ---------------------------------------------------------------------------

// TestGetOperationsAfterBasic verifies the core scenario from the task
// specification:
//
//	Current Version: 10
//	Request: Get operations after version 7
//	Expected: Operations for versions 8, 9, and 10
func TestGetOperationsAfterBasic(t *testing.T) {
	room := NewRoom("get-ops-after-test")

	// Apply 10 operations to reach version 10
	for i := 1; i <= 10; i++ {
		op := Operation{
			ID:       fmt.Sprintf("op-%02d", i),
			Type:     InsertOperation,
			Position: 0,
			Text:     "x",
		}
		_, _ = room.ApplyOperation(op)
	}

	// Verify current version
	if room.GetVersion() != 10 {
		t.Fatalf("expected version 10, got %d", room.GetVersion())
	}

	// Get operations after version 7
	ops := room.GetOperationsAfter(7)

	// Should return versions 8, 9, 10
	if len(ops) != 3 {
		t.Fatalf("expected 3 operations after version 7, got %d", len(ops))
	}

	// Verify the versions are correct
	for i, op := range ops {
		expectedVersion := 8 + i
		if op.Version != expectedVersion {
			t.Errorf("op %d: expected version %d, got %d", i, expectedVersion, op.Version)
		}
	}

	// Verify the operation details
	if ops[0].Operation.ID != "op-08" {
		t.Errorf("expected op[0] ID 'op-08', got '%s'", ops[0].Operation.ID)
	}
	if ops[1].Operation.ID != "op-09" {
		t.Errorf("expected op[1] ID 'op-09', got '%s'", ops[1].Operation.ID)
	}
	if ops[2].Operation.ID != "op-10" {
		t.Errorf("expected op[2] ID 'op-10', got '%s'", ops[2].Operation.ID)
	}
}

// TestGetOperationsAfterVersion0 verifies that version 0 returns all operations.
func TestGetOperationsAfterVersion0(t *testing.T) {
	room := NewRoom("get-ops-after-v0-test")

	// Apply 5 operations
	for i := 1; i <= 5; i++ {
		op := Operation{
			ID:   fmt.Sprintf("op-%d", i),
			Type: InsertOperation,
			Text: "x",
		}
		_, _ = room.ApplyOperation(op)
	}

	// Get operations after version 0 (should return all)
	ops := room.GetOperationsAfter(0)

	if len(ops) != 5 {
		t.Fatalf("expected 5 operations after version 0, got %d", len(ops))
	}
}

// TestGetOperationsAfterNegativeVersion verifies that negative versions
// return all operations.
func TestGetOperationsAfterNegativeVersion(t *testing.T) {
	room := NewRoom("get-ops-after-negative-test")

	// Apply 3 operations
	for i := 1; i <= 3; i++ {
		op := Operation{
			ID:   fmt.Sprintf("op-%d", i),
			Type: InsertOperation,
			Text: "x",
		}
		_, _ = room.ApplyOperation(op)
	}

	// Get operations after version -1 (should return all)
	ops := room.GetOperationsAfter(-1)

	if len(ops) != 3 {
		t.Fatalf("expected 3 operations after version -1, got %d", len(ops))
	}
}

// TestGetOperationsAfterCurrentVersion verifies that requesting operations
// after the current version returns an empty slice.
func TestGetOperationsAfterCurrentVersion(t *testing.T) {
	room := NewRoom("get-ops-after-current-test")

	// Apply 5 operations
	for i := 1; i <= 5; i++ {
		op := Operation{
			ID:   fmt.Sprintf("op-%d", i),
			Type: InsertOperation,
			Text: "x",
		}
		_, _ = room.ApplyOperation(op)
	}

	// Get operations after current version (5)
	ops := room.GetOperationsAfter(5)

	if len(ops) != 0 {
		t.Errorf("expected 0 operations after current version, got %d", len(ops))
	}

	// Get operations after a version greater than current
	ops = room.GetOperationsAfter(10)

	if len(ops) != 0 {
		t.Errorf("expected 0 operations after version greater than current, got %d", len(ops))
	}
}

// TestGetOperationsAfterEmptyHistory verifies that requesting operations
// after any version on a room with empty history returns an empty slice.
func TestGetOperationsAfterEmptyHistory(t *testing.T) {
	room := NewRoom("get-ops-after-empty-test")

	ops := room.GetOperationsAfter(0)

	if len(ops) != 0 {
		t.Errorf("expected 0 operations from empty history, got %d", len(ops))
	}

	ops = room.GetOperationsAfter(5)

	if len(ops) != 0 {
		t.Errorf("expected 0 operations from empty history after version 5, got %d", len(ops))
	}
}

// TestGetOperationsAfterPartialRange verifies getting a partial range
// of operations.
func TestGetOperationsAfterPartialRange(t *testing.T) {
	room := NewRoom("get-ops-after-partial-test")

	// Apply 10 operations
	for i := 1; i <= 10; i++ {
		op := Operation{
			ID:   fmt.Sprintf("op-%02d", i),
			Type: InsertOperation,
			Text: "x",
		}
		_, _ = room.ApplyOperation(op)
	}

	// Get operations after version 9 (should return only version 10)
	ops := room.GetOperationsAfter(9)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation after version 9, got %d", len(ops))
	}
	if ops[0].Version != 10 {
		t.Errorf("expected version 10, got %d", ops[0].Version)
	}

	// Get operations after version 5 (should return versions 6-10)
	ops = room.GetOperationsAfter(5)

	if len(ops) != 5 {
		t.Fatalf("expected 5 operations after version 5, got %d", len(ops))
	}
}

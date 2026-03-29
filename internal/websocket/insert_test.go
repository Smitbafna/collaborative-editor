package websocket

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ApplyInsert tests
// ---------------------------------------------------------------------------

// TestRoomApplyInsert verifies that ApplyInsert correctly inserts text at
// the given position, matching the task specification exactly.
//
// Initial document: "Hello World"
// INSERT
// Position: 6
// Text: "beautiful "
// Result: "Hello beautiful World"
//
// Mathematically:
//   content[:6]  = "Hello "
//   text         = "beautiful "
//   content[6:]  = "World"
//   result       = "Hello " + "beautiful " + "World" = "Hello beautiful World"
func TestRoomApplyInsert(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	updated := room.ApplyInsert(6, "beautiful ")

	expected := "Hello beautiful World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}

	// Verify the room content was actually updated
	if got := room.GetContent(); got != expected {
		t.Errorf("room content expected '%s', got '%s'", expected, got)
	}
}

// TestRoomApplyInsertAtBeginning verifies inserting at position 0.
//
// Initial content: "World"
// Insert "Hello " at position 0
// Expected: "Hello World"
func TestRoomApplyInsertAtBeginning(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("World")

	updated := room.ApplyInsert(0, "Hello ")

	expected := "Hello World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyInsertAtEnd verifies inserting at the end of the content.
//
// Initial content: "Hello"
// Insert " World" at position 5
// Expected: "Hello World"
func TestRoomApplyInsertAtEnd(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyInsert(5, " World")

	expected := "Hello World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyInsertOnEmptyContent verifies inserting into empty content.
//
// Initial content: ""
// Insert "Hello" at position 0
// Expected: "Hello"
func TestRoomApplyInsertOnEmptyContent(t *testing.T) {
	room := NewRoom("test-room")

	updated := room.ApplyInsert(0, "Hello")

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyInsertInvalidPosition verifies that an insert with
// an invalid position (beyond content length) does not modify the content.
//
// Initial content: "Hello"
// Insert "X" at position 10 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyInsertInvalidPosition(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyInsert(10, "X")

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyInsertNegativePosition verifies that an insert with
// a negative position does not modify the content.
//
// Initial content: "Hello"
// Insert "X" at position -1 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyInsertNegativePosition(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyInsert(-1, "X")

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyInsertEmptyText verifies that inserting empty text
// does not modify the content.
//
// Initial content: "Hello"
// Insert "" at position 2
// Expected: "Hello" (unchanged)
func TestRoomApplyInsertEmptyText(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyInsert(2, "")

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyInsertMultiple verifies that multiple ApplyInsert calls
// can be chained to build up content.
//
// Insert "Hello" at 0 → "Hello"
// Insert " " at 5 → "Hello "
// Insert "World" at 6 → "Hello World"
func TestRoomApplyInsertMultiple(t *testing.T) {
	room := NewRoom("test-room")

	room.ApplyInsert(0, "Hello")
	room.ApplyInsert(5, " ")
	room.ApplyInsert(6, "World")

	expected := "Hello World"
	if got := room.GetContent(); got != expected {
		t.Errorf("expected '%s', got '%s'", expected, got)
	}
}

// TestRoomApplyInsertConcurrent verifies that ApplyInsert can be called
// concurrently from multiple goroutines without data races.
func TestRoomApplyInsertConcurrent(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("")

	const goroutines = 10
	const operationsPerGoroutine = 50

	done := make(chan bool)

	for range goroutines {
		go func() {
			for range operationsPerGoroutine {
				room.ApplyInsert(0, "a")
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for range goroutines {
		<-done
	}

	// Content should have all the inserted characters
	expectedLen := goroutines * operationsPerGoroutine
	if got := len(room.GetContent()); got != expectedLen {
		t.Errorf("expected content length %d, got %d", expectedLen, got)
	}
}

// TestRoomApplyInsertConsistentWithApplyOperation verifies that ApplyInsert
// produces the same result as ApplyOperation with an InsertOperation.
func TestRoomApplyInsertConsistentWithApplyOperation(t *testing.T) {
	room1 := NewRoom("test-room-1")
	room1.SetContent("Hello World")

	room2 := NewRoom("test-room-2")
	room2.SetContent("Hello World")

	// Apply via ApplyInsert
	result1 := room1.ApplyInsert(6, "beautiful ")

	// Apply via ApplyOperation
	op := Operation{
		Type:     InsertOperation,
		Position: 6,
		Text:     "beautiful ",
	}
	result2 := room2.ApplyOperation(op)

	if result1 != result2 {
		t.Errorf("ApplyInsert and ApplyOperation produced different results: '%s' vs '%s'", result1, result2)
	}

	if room1.GetContent() != room2.GetContent() {
		t.Errorf("room contents differ: '%s' vs '%s'", room1.GetContent(), room2.GetContent())
	}
}

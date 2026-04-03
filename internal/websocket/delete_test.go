package websocket

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ApplyDelete tests
// ---------------------------------------------------------------------------

// TestRoomApplyDelete verifies that ApplyDelete correctly deletes characters
// at the given position, matching the task specification exactly.
//
// Initial document: "Hello beautiful World"
// DELETE
// Position: 6
// Length: 10
// Result: "Hello World"
//
// Mathematically:
//   content[:6]      = "Hello "
//   content[6+10:]   = content[16:] = "World"
//   result           = "Hello " + "World" = "Hello World"
func TestRoomApplyDelete(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello beautiful World")

	updated := room.ApplyDelete(6, 10)

	expected := "Hello World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}

	// Verify the room content was actually updated
	if got := room.GetContent(); got != expected {
		t.Errorf("room content expected '%s', got '%s'", expected, got)
	}
}

// TestRoomApplyDeleteFromBeginning verifies deleting from position 0.
//
// Initial content: "Hello World"
// Delete 6 characters at position 0
// Expected: "World"
func TestRoomApplyDeleteFromBeginning(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	updated := room.ApplyDelete(0, 6)

	expected := "World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteToEnd verifies deleting all characters from a position
// to the end of the content.
//
// Initial content: "Hello World"
// Delete 6 characters at position 5
// Expected: "Hello"
func TestRoomApplyDeleteToEnd(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	updated := room.ApplyDelete(5, 6)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteSingleCharacter verifies deleting a single character.
//
// Initial content: "Hello World"
// Delete 1 character at position 5 (the space)
// Expected: "HelloWorld"
func TestRoomApplyDeleteSingleCharacter(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	updated := room.ApplyDelete(5, 1)

	expected := "HelloWorld"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteEntireContent verifies deleting the entire content.
//
// Initial content: "Hello"
// Delete 5 characters at position 0
// Expected: ""
func TestRoomApplyDeleteEntireContent(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyDelete(0, 5)

	expected := ""
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteInvalidPosition verifies that a delete with
// an invalid position (beyond content length) does not modify the content.
//
// Initial content: "Hello"
// Delete 1 character at position 10 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyDeleteInvalidPosition(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyDelete(10, 1)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteNegativePosition verifies that a delete with
// a negative position does not modify the content.
//
// Initial content: "Hello"
// Delete 1 character at position -1 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyDeleteNegativePosition(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyDelete(-1, 1)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteZeroLength verifies that a delete with length 0
// does not modify the content.
//
// Initial content: "Hello"
// Delete 0 characters at position 2
// Expected: "Hello" (unchanged)
func TestRoomApplyDeleteZeroLength(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyDelete(2, 0)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteNegativeLength verifies that a delete with a negative
// length does not modify the content.
//
// Initial content: "Hello"
// Delete -1 characters at position 2 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyDeleteNegativeLength(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyDelete(2, -1)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyDeletePastEnd verifies that a delete with length extending
// past the end of the content only deletes up to the end.
//
// Initial content: "Hello"
// Delete 100 characters at position 3 (only 2 characters remain)
// Expected: "Hel"
func TestRoomApplyDeletePastEnd(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	updated := room.ApplyDelete(3, 100)

	expected := "Hel"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteOnEmptyContent verifies deleting from empty content.
//
// Initial content: ""
// Delete 1 character at position 0 (invalid — empty content)
// Expected: "" (unchanged)
func TestRoomApplyDeleteOnEmptyContent(t *testing.T) {
	room := NewRoom("test-room")

	updated := room.ApplyDelete(0, 1)

	expected := ""
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyDeleteMultiple verifies that multiple ApplyDelete calls
// can be chained.
//
// Initial: "Hello beautiful World"
// Delete 10 at 6 → "Hello World"
// Delete 6 at 0 → "World"
// Delete 5 at 0 → ""
func TestRoomApplyDeleteMultiple(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello beautiful World")

	room.ApplyDelete(6, 10)  // "Hello World"
	room.ApplyDelete(0, 6)   // "World"
	room.ApplyDelete(0, 5)   // ""

	expected := ""
	if got := room.GetContent(); got != expected {
		t.Errorf("expected '%s', got '%s'", expected, got)
	}
}

// TestRoomApplyDeleteConcurrent verifies that ApplyDelete can be called
// concurrently from multiple goroutines without data races.
func TestRoomApplyDeleteConcurrent(t *testing.T) {
	room := NewRoom("test-room")

	// Build up content: 500 characters of "a"
	content := ""
	for range 500 {
		content += "a"
	}
	room.SetContent(content)

	const goroutines = 10
	const operationsPerGoroutine = 50

	done := make(chan bool)

	for range goroutines {
		go func() {
			for range operationsPerGoroutine {
				room.ApplyDelete(0, 1)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for range goroutines {
		<-done
	}

	// Content should have all characters deleted
	expectedLen := 500 - (goroutines * operationsPerGoroutine)
	if got := len(room.GetContent()); got != expectedLen {
		t.Errorf("expected content length %d, got %d", expectedLen, got)
	}
}

// TestRoomApplyDeleteConsistentWithApplyOperation verifies that ApplyDelete
// produces the same result as ApplyOperation with a DeleteOperation.
func TestRoomApplyDeleteConsistentWithApplyOperation(t *testing.T) {
	room1 := NewRoom("test-room-1")
	room1.SetContent("Hello beautiful World")

	room2 := NewRoom("test-room-2")
	room2.SetContent("Hello beautiful World")

	// Apply via ApplyDelete
	result1 := room1.ApplyDelete(6, 10)

	// Apply via ApplyOperation
	op := Operation{
		Type:     DeleteOperation,
		Position: 6,
		Length:   10,
	}
	result2, _ := room2.ApplyOperation(op)

	if result1 != result2 {
		t.Errorf("ApplyDelete and ApplyOperation produced different results: '%s' vs '%s'", result1, result2)
	}

	if room1.GetContent() != room2.GetContent() {
		t.Errorf("room contents differ: '%s' vs '%s'", room1.GetContent(), room2.GetContent())
	}
}

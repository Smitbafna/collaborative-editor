package websocket

import (
	"testing"
)

// helper to build an operation with the given parameters
func op(tpe OperationType, pos int, txt string, length int) Operation {
	return Operation{
		Type:     tpe,
		Position: pos,
		Text:     txt,
		Length:   length,
	}
}

// helper to build an operation with all fields including ID
func opWithID(id string, tpe OperationType, pos int, txt string, length int) Operation {
	return Operation{
		ID:       id,
		Type:     tpe,
		Position: pos,
		Text:     txt,
		Length:   length,
	}
}

// ---------------------------------------------------------------------------
// INSERT vs INSERT
// ---------------------------------------------------------------------------

func TestTransformInsertInsertSamePosition(t *testing.T) {
	// Client wants to insert "B" at position 0
	// Server already inserted "A" at position 0
	// With no IDs, B's ID (empty) is not < A's ID (empty), so A wins
	incoming := op(InsertOperation, 0, "B", 0)
	applied := op(InsertOperation, 0, "A", 0)
	got := Transform(incoming, applied)

	if got.Position != 1 {
		t.Errorf("expected position 1, got %d", got.Position)
	}
	if got.Text != "B" {
		t.Errorf("expected text 'B', got %q", got.Text)
	}
}

func TestTransformInsertInsertSamePositionIncomingWins(t *testing.T) {
	// Both inserts target position 5
	// incoming ID "aaa" < applied ID "bbb", so incoming wins
	// incoming should insert at position 5 (before applied)
	incoming := opWithID("aaa", InsertOperation, 5, " beautiful", 0)
	applied := opWithID("bbb", InsertOperation, 5, " everyone", 0)
	got := Transform(incoming, applied)

	if got.Position != 5 {
		t.Errorf("expected position 5, got %d", got.Position)
	}
	if got.Text != " beautiful" {
		t.Errorf("expected text ' beautiful', got %q", got.Text)
	}
	if got.ID != "aaa" {
		t.Errorf("expected ID 'aaa', got %q", got.ID)
	}
}

func TestTransformInsertInsertSamePositionAppliedWins(t *testing.T) {
	// Both inserts target position 5
	// incoming ID "bbb" > applied ID "aaa", so applied wins
	// incoming should move to after applied text
	incoming := opWithID("bbb", InsertOperation, 5, " beautiful", 0)
	applied := opWithID("aaa", InsertOperation, 5, " everyone", 0)
	got := Transform(incoming, applied)

	// applied.Position (5) + len(applied.Text) (9) = 14
	if got.Position != 14 {
		t.Errorf("expected position 14, got %d", got.Position)
	}
	if got.Text != " beautiful" {
		t.Errorf("expected text ' beautiful', got %q", got.Text)
	}
	if got.ID != "bbb" {
		t.Errorf("expected ID 'bbb', got %q", got.ID)
	}
}

func TestTransformInsertInsertIncomingBeforeApplied(t *testing.T) {
	// incoming insert at position 2, applied insert at position 5
	// incoming is before applied, so it stays at its original position
	incoming := op(InsertOperation, 2, "B", 0)
	applied := op(InsertOperation, 5, "A", 0)
	got := Transform(incoming, applied)

	if got.Position != 2 {
		t.Errorf("expected position 2, got %d", got.Position)
	}
}

func TestTransformInsertInsertIncomingAfterApplied(t *testing.T) {
	// incoming insert at position 8, applied insert at position 5 of len 2
	// incoming shifts right by len("AA")
	incoming := op(InsertOperation, 8, "B", 0)
	applied := op(InsertOperation, 5, "AA", 0)
	got := Transform(incoming, applied)

	if got.Position != 10 {
		t.Errorf("expected position 10, got %d", got.Position)
	}
}

// ---------------------------------------------------------------------------
// INSERT vs DELETE
// ---------------------------------------------------------------------------

func TestTransformInsertDeleteBeforeDeletedRange(t *testing.T) {
	// Insert at position 2
	// Delete already removed [5, 7)
	// Insert is before the delete, so position is unchanged
	incoming := op(InsertOperation, 2, "B", 0)
	applied := op(DeleteOperation, 5, "", 2)
	got := Transform(incoming, applied)

	if got.Position != 2 {
		t.Errorf("expected position 2, got %d", got.Position)
	}
}

func TestTransformInsertDeleteAfterDeletedRange(t *testing.T) {
	// Insert at position 10
	// Delete already removed [5, 7)
	// Insert is after, so shift left by 2
	incoming := op(InsertOperation, 10, "B", 0)
	applied := op(DeleteOperation, 5, "", 2)
	got := Transform(incoming, applied)

	if got.Position != 8 {
		t.Errorf("expected position 8, got %d", got.Position)
	}
}

func TestTransformInsertDeleteWithinDeletedRange(t *testing.T) {
	// Insert at position 6 (inside deleted range [5, 7))
	// Should move to the start of the deleted range
	incoming := op(InsertOperation, 6, "B", 0)
	applied := op(DeleteOperation, 5, "", 2)
	got := Transform(incoming, applied)

	if got.Position != 5 {
		t.Errorf("expected position 5, got %d", got.Position)
	}
}

func TestTransformInsertDeleteAtStartOfDeletedRange(t *testing.T) {
	// Insert exactly at start of deleted range
	incoming := op(InsertOperation, 5, "B", 0)
	applied := op(DeleteOperation, 5, "", 2)
	got := Transform(incoming, applied)

	if got.Position != 5 {
		t.Errorf("expected position 5, got %d", got.Position)
	}
}

func TestTransformInsertDeleteAtEndOfDeletedRange(t *testing.T) {
	// Insert at position 7 (end of deleted range [5, 7))
	// This is treated as "after" the deleted range
	incoming := op(InsertOperation, 7, "B", 0)
	applied := op(DeleteOperation, 5, "", 2)
	got := Transform(incoming, applied)

	if got.Position != 5 {
		t.Errorf("expected position 5 (shift left by 2), got %d", got.Position)
	}
}

func TestTransformInsertDeleteStep4Example(t *testing.T) {
	// Initial document: "Hello World"
	// A: DELETE(position=5, length=6) removes " World"
	// B: INSERT(" beautiful", position=11)
	// After applying A first, B's position should be transformed to 5
	incoming := op(InsertOperation, 11, " beautiful", 0)
	applied := op(DeleteOperation, 5, "", 6)
	got := Transform(incoming, applied)

	if got.Position != 5 {
		t.Errorf("expected position 5, got %d", got.Position)
	}
	if got.Text != " beautiful" {
		t.Errorf("expected text ' beautiful', got %q", got.Text)
	}
}

// ---------------------------------------------------------------------------
// DELETE vs INSERT
// ---------------------------------------------------------------------------

func TestTransformDeleteInsertBeforeApplied(t *testing.T) {
	// Delete [2, 5)
	// Insert at position 6
	// Delete is before the insert, so unchanged
	incoming := op(DeleteOperation, 2, "", 3)
	applied := op(InsertOperation, 6, "AAA", 0)
	got := Transform(incoming, applied)

	if got.Position != 2 || got.Length != 3 {
		t.Errorf("expected pos=2 len=3, got pos=%d len=%d", got.Position, got.Length)
	}
}

func TestTransformDeleteInsertAfterApplied(t *testing.T) {
	// Delete [8, 11)
	// Insert at position 6 of len 3
	// Delete shifts right by 3
	incoming := op(DeleteOperation, 8, "", 3)
	applied := op(InsertOperation, 6, "AAA", 0)
	got := Transform(incoming, applied)

	if got.Position != 11 || got.Length != 3 {
		t.Errorf("expected pos=11 len=3, got pos=%d len=%d", got.Position, got.Length)
	}
}

func TestTransformDeleteInsertAtAppliedPosition(t *testing.T) {
	// Delete [6, 9)
	// Insert at position 6
	// After insert, the delete starts at 6+3=9
	incoming := op(DeleteOperation, 6, "", 3)
	applied := op(InsertOperation, 6, "AAA", 0)
	got := Transform(incoming, applied)

	if got.Position != 9 || got.Length != 3 {
		t.Errorf("expected pos=9 len=3, got pos=%d len=%d", got.Position, got.Length)
	}
}

func TestTransformDeleteInsertStep4Example(t *testing.T) {
	// Initial document: "Hello World"
	// A: INSERT(" beautiful", position=5) is applied
	// B: DELETE(position=5, length=6) arrives
	// B should NOT delete the inserted text; it should delete what was at
	// position 5 originally (" World"), so after transformation the delete
	// shifts: 5 + len(" beautiful") = 15, length stays 6
	incoming := op(DeleteOperation, 5, "", 6)
	applied := op(InsertOperation, 5, " beautiful", 0)
	got := Transform(incoming, applied)

	if got.Position != 15 || got.Length != 6 {
		t.Errorf("expected pos=15 len=6, got pos=%d len=%d", got.Position, got.Length)
	}
}

// ---------------------------------------------------------------------------
// DELETE vs DELETE
// ---------------------------------------------------------------------------

func TestTransformDeleteDeleteCompletelyBefore(t *testing.T) {
	// Incoming delete [2, 5)
	// Applied delete [8, 11)
	// No overlap, shift left by 3
	incoming := op(DeleteOperation, 2, "", 3)
	applied := op(DeleteOperation, 8, "", 3)
	got := Transform(incoming, applied)

	if got.Type != DeleteOperation {
		t.Errorf("expected type delete, got %q", got.Type)
	}
	if got.Position != 2 || got.Length != 3 {
		t.Errorf("expected pos=2 len=3, got pos=%d len=%d", got.Position, got.Length)
	}
}

func TestTransformDeleteDeleteCompletelyAfter(t *testing.T) {
	// Incoming delete [10, 13)
	// Applied delete [5, 8)
	// Shift incoming left by 3
	incoming := op(DeleteOperation, 10, "", 3)
	applied := op(DeleteOperation, 5, "", 3)
	got := Transform(incoming, applied)

	if got.Type != DeleteOperation {
		t.Errorf("expected type delete, got %q", got.Type)
	}
	if got.Position != 7 || got.Length != 3 {
		t.Errorf("expected pos=7 len=3, got pos=%d len=%d", got.Position, got.Length)
	}
}

func TestTransformDeleteDeleteOverlapsStart(t *testing.T) {
	// Incoming delete [3, 9)  (len 6)
	// Applied delete [5, 10) (len 5)
	// Overlap: [5, 9), which is 4 chars.
	// After transformation: still need to remove [3, 5) → len 2
	incoming := op(DeleteOperation, 3, "", 6)
	applied := op(DeleteOperation, 5, "", 5)
	got := Transform(incoming, applied)

	if got.Type != DeleteOperation {
		t.Errorf("expected type delete, got %q", got.Type)
	}
	if got.Position != 3 || got.Length != 2 {
		t.Errorf("expected pos=3 len=2, got pos=%d len=%d", got.Position, got.Length)
	}
}

func TestTransformDeleteDeleteOverlapsEnd(t *testing.T) {
	// Incoming delete [5, 12)
	// Applied delete [3, 8)
	// Remaining: [8, 12) → len 4
	incoming := op(DeleteOperation, 5, "", 7)
	applied := op(DeleteOperation, 3, "", 5)
	got := Transform(incoming, applied)

	if got.Type != DeleteOperation {
		t.Errorf("expected type delete, got %q", got.Type)
	}
	if got.Position != 3 || got.Length != 4 {
		t.Errorf("expected pos=3 len=4, got pos=%d len=%d", got.Position, got.Length)
	}
}

func TestTransformDeleteDeleteCompletelyWithin(t *testing.T) {
	// Incoming delete [6, 8)
	// Applied delete [4, 10)
	// Incoming is fully contained → becomes NO-OP
	incoming := op(DeleteOperation, 6, "", 2)
	applied := op(DeleteOperation, 4, "", 6)
	got := Transform(incoming, applied)

	if got.Type != NoOpOperation {
		t.Errorf("expected type noop, got %q", got.Type)
	}
	if got.Position != 0 || got.Length != 0 {
		t.Errorf("expected pos=0 len=0 for noop, got pos=%d len=%d", got.Position, got.Length)
	}
	if got.ID != incoming.ID {
		t.Errorf("expected ID to be preserved, got %q", got.ID)
	}
}

func TestTransformDeleteDeleteSameRange(t *testing.T) {
	// Both delete the exact same range
	incoming := op(DeleteOperation, 5, "", 3)
	applied := op(DeleteOperation, 5, "", 3)
	got := Transform(incoming, applied)

	if got.Type != NoOpOperation {
		t.Errorf("expected type noop, got %q", got.Type)
	}
	if got.Position != 0 || got.Length != 0 {
		t.Errorf("expected pos=0 len=0 for noop, got pos=%d len=%d", got.Position, got.Length)
	}
	if got.ID != incoming.ID {
		t.Errorf("expected ID to be preserved, got %q", got.ID)
	}
}

func TestTransformDeleteDeleteStep7Example(t *testing.T) {
	// Initial document: "Hello beautiful World"
	// A: DELETE(position=5, length=10) removes "beautiful "
	// B: DELETE(position=5, length=6) removes "beauti"
	// After A is applied, B's target is completely gone → NO-OP
	incoming := op(DeleteOperation, 5, "", 6)
	applied := op(DeleteOperation, 5, "", 10)
	got := Transform(incoming, applied)

	if got.Type != NoOpOperation {
		t.Errorf("expected type noop, got %q", got.Type)
	}
	if got.Position != 0 || got.Length != 0 {
		t.Errorf("expected pos=0 len=0 for noop, got pos=%d len=%d", got.Position, got.Length)
	}
	if got.ID != incoming.ID {
		t.Errorf("expected ID to be preserved, got %q", got.ID)
	}
}

func TestTransformDeleteDeletePartialOverlapStep8Example(t *testing.T) {
	// Initial document: "abcdefghij" (length 10)
	// A: DELETE(position=2, length=5) deletes [2, 7) = "cdefg"
	// B: DELETE(position=5, length=4) wants to delete [5, 9) = "fghi"
	//
	// After applying A: "ab" + "hij" = "abhij"
	//
	// B's delete [5, 9) overlaps with A's delete [2, 7):
	//   - The part [5, 7) ("fg") was already deleted by A
	//   - The remaining part [7, 9) ("hi") still exists at position 2 in the new document
	//
	// Expected: B becomes DELETE(position=2, length=2)
	incoming := op(DeleteOperation, 5, "", 4)
	applied := op(DeleteOperation, 2, "", 5)
	got := Transform(incoming, applied)

	if got.Type != DeleteOperation {
		t.Errorf("expected type delete, got %q", got.Type)
	}
	if got.Position != 2 || got.Length != 2 {
		t.Errorf("expected pos=2 len=2, got pos=%d len=%d", got.Position, got.Length)
	}
	if got.ID != incoming.ID {
		t.Errorf("expected ID to be preserved, got %q", got.ID)
	}
}

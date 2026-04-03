package websocket

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helper: apply an operation to a document string (lock-free, for testing)
// ---------------------------------------------------------------------------

// applyOpToContent applies an operation to a document string and returns the
// resulting content. This mirrors Room.applyInsert / Room.applyDelete but
// works on a plain string so tests can verify end-to-end consistency without
// a full Room.
func applyOpToContent(content string, op Operation) string {
	switch op.Type {
	case InsertOperation:
		if op.Position < 0 || op.Position > len(content) {
			return content
		}
		return content[:op.Position] + op.Text + content[op.Position:]
	case DeleteOperation:
		if op.Position < 0 || op.Position >= len(content) {
			return content
		}
		if op.Length <= 0 {
			return content
		}
		end := op.Position + op.Length
		if end > len(content) {
			end = len(content)
		}
		return content[:op.Position] + content[end:]
	case NoOpOperation:
		return content
	default:
		return content
	}
}

// ---------------------------------------------------------------------------
// INSERT + INSERT  —  two inserts at the same position
// ---------------------------------------------------------------------------

// TestTransformPairInsertInsertSamePosition verifies the INSERT+INSERT pair
// where both operations insert at the same position.
//
// Initial document: "Hello"
// A: insert("A", 5)
// B: insert("B", 5)
//
// Expected: a deterministic result every time. Because both inserts target
// the same position, the tie-breaker (operation ID) determines ordering.
// Whichever operation has the smaller ID is applied first, so the final
// document is the same regardless of which operation is applied first.
func TestTransformPairInsertInsertSamePosition(t *testing.T) {
	initial := "Hello"

	opA := opWithID("aaa", InsertOperation, 5, "A", 0)
	opB := opWithID("bbb", InsertOperation, 5, "B", 0)

	// --- Order 1: A first, then B (transformed) ---
	doc1 := applyOpToContent(initial, opA)
	transformedB := Transform(opB, opA)
	doc1 = applyOpToContent(doc1, transformedB)

	// --- Order 2: B first, then A (transformed) ---
	doc2 := applyOpToContent(initial, opB)
	transformedA := Transform(opA, opB)
	doc2 = applyOpToContent(doc2, transformedA)

	// Both orders must produce the same deterministic result.
	if doc1 != doc2 {
		t.Errorf("INSERT+INSERT same position: order-dependent results: %q vs %q", doc1, doc2)
	}

	// With ID "aaa" < "bbb", A wins the tie-break and goes first.
	// Final document should be "HelloAB".
	expected := "HelloAB"
	if doc1 != expected {
		t.Errorf("expected %q, got %q (order1) and %q (order2)", expected, doc1, doc2)
	}
}

// TestTransformPairInsertInsertSamePositionReverseIDs tests the same scenario
// but with reversed IDs to confirm the tie-breaker works in both directions.
func TestTransformPairInsertInsertSamePositionReverseIDs(t *testing.T) {
	initial := "Hello"

	opA := opWithID("bbb", InsertOperation, 5, "A", 0)
	opB := opWithID("aaa", InsertOperation, 5, "B", 0)

	// --- Order 1: A first, then B (transformed) ---
	doc1 := applyOpToContent(initial, opA)
	transformedB := Transform(opB, opA)
	doc1 = applyOpToContent(doc1, transformedB)

	// --- Order 2: B first, then A (transformed) ---
	doc2 := applyOpToContent(initial, opB)
	transformedA := Transform(opA, opB)
	doc2 = applyOpToContent(doc2, transformedA)

	if doc1 != doc2 {
		t.Errorf("INSERT+INSERT reverse IDs: order-dependent results: %q vs %q", doc1, doc2)
	}

	// With ID "aaa" < "bbb", B wins the tie-break and goes first.
	// Final document should be "HelloBA".
	expected := "HelloBA"
	if doc1 != expected {
		t.Errorf("expected %q, got %q (order1) and %q (order2)", expected, doc1, doc2)
	}
}

// ---------------------------------------------------------------------------
// INSERT after INSERT  —  two inserts at the same position, different text
// ---------------------------------------------------------------------------

// TestTransformPairInsertAfterInsert verifies the INSERT-after-INSERT pair
// where both operations insert at the same position with different text.
//
// Initial document: "Hello"
// A: insert(" World", 5)
// B: insert("!", 5)
//
// Expected: "Hello World!" (or the deterministic reverse ordering depending
// on the tie-breaker). With ID "aaa" < "bbb", A wins and goes first.
func TestTransformPairInsertAfterInsert(t *testing.T) {
	initial := "Hello"

	opA := opWithID("aaa", InsertOperation, 5, " World", 0)
	opB := opWithID("bbb", InsertOperation, 5, "!", 0)

	// --- Order 1: A first, then B (transformed) ---
	doc1 := applyOpToContent(initial, opA)
	transformedB := Transform(opB, opA)
	doc1 = applyOpToContent(doc1, transformedB)

	// --- Order 2: B first, then A (transformed) ---
	doc2 := applyOpToContent(initial, opB)
	transformedA := Transform(opA, opB)
	doc2 = applyOpToContent(doc2, transformedA)

	// Both orders must produce the same deterministic result.
	if doc1 != doc2 {
		t.Errorf("INSERT after INSERT: order-dependent results: %q vs %q", doc1, doc2)
	}

	// A wins the tie-break (aaa < bbb), so " World" goes before "!".
	expected := "Hello World!"
	if doc1 != expected {
		t.Errorf("expected %q, got %q (order1) and %q (order2)", expected, doc1, doc2)
	}
}

// TestTransformPairInsertAfterInsertReverseIDs tests the reverse ID ordering.
func TestTransformPairInsertAfterInsertReverseIDs(t *testing.T) {
	initial := "Hello"

	opA := opWithID("bbb", InsertOperation, 5, " World", 0)
	opB := opWithID("aaa", InsertOperation, 5, "!", 0)

	// --- Order 1: A first, then B (transformed) ---
	doc1 := applyOpToContent(initial, opA)
	transformedB := Transform(opB, opA)
	doc1 = applyOpToContent(doc1, transformedB)

	// --- Order 2: B first, then A (transformed) ---
	doc2 := applyOpToContent(initial, opB)
	transformedA := Transform(opA, opB)
	doc2 = applyOpToContent(doc2, transformedA)

	if doc1 != doc2 {
		t.Errorf("INSERT after INSERT reverse IDs: order-dependent results: %q vs %q", doc1, doc2)
	}

	// B wins the tie-break (aaa < bbb), so "!" goes before " World".
	expected := "Hello! World"
	if doc1 != expected {
		t.Errorf("expected %q, got %q (order1) and %q (order2)", expected, doc1, doc2)
	}
}

// ---------------------------------------------------------------------------
// INSERT + DELETE  —  insert arrives after a delete has been applied
// ---------------------------------------------------------------------------

// TestTransformPairInsertDelete verifies the INSERT+DELETE pair where the
// insert arrives after the delete has already been applied.
//
// Initial document: "Hello World"
// Delete: position=5, length=6  (removes " World")
// Insert: position=11, text="X"
//
// Expected: "HelloX"
//
// The delete removes " World" (positions 5-10). The insert at position 11
// is at the end of the original document. After the delete, the document
// is "Hello" (length 5). The insert position 11 >= 5+6=11, so it shifts
// left by 6 to position 5. Result: "Hello" + "X" = "HelloX".
func TestTransformPairInsertDelete(t *testing.T) {
	initial := "Hello World"

	deleteOp := opWithID("del-1", DeleteOperation, 5, "", 6)
	insertOp := opWithID("ins-1", InsertOperation, 11, "X", 0)

	// --- Order 1: Delete first, then insert (transformed) ---
	doc1 := applyOpToContent(initial, deleteOp)
	transformedIns := Transform(insertOp, deleteOp)
	doc1 = applyOpToContent(doc1, transformedIns)

	// Verify the transformed insert position
	if transformedIns.Position != 5 {
		t.Errorf("transformed insert position: expected 5, got %d", transformedIns.Position)
	}

	// --- Order 2: Insert first, then delete (transformed) ---
	doc2 := applyOpToContent(initial, insertOp)
	transformedDel := Transform(deleteOp, insertOp)
	doc2 = applyOpToContent(doc2, transformedDel)

	// Verify the transformed delete is unchanged (insert is after the delete)
	if transformedDel.Position != 5 || transformedDel.Length != 6 {
		t.Errorf("transformed delete: expected pos=5 len=6, got pos=%d len=%d",
			transformedDel.Position, transformedDel.Length)
	}

	// Both orders must produce the same result.
	expected := "HelloX"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// INSERT inside DELETE  —  insert position falls within the deleted range
// ---------------------------------------------------------------------------

// TestTransformPairInsertInsideDelete verifies the INSERT-inside-DELETE pair
// where the insert position falls within the deleted range.
//
// Initial document: "Hello World"
// Delete: position=5, length=6  (removes " World")
// Insert: position=7, text="X"
//
// Expected: "HelloX"
//
// The insert at position 7 falls within the deleted range [5, 11). Per the
// transform policy, the insert moves to the start of the deleted range
// (position 5). Result: "Hello" + "X" = "HelloX".
func TestTransformPairInsertInsideDelete(t *testing.T) {
	initial := "Hello World"

	deleteOp := opWithID("del-1", DeleteOperation, 5, "", 6)
	insertOp := opWithID("ins-1", InsertOperation, 7, "X", 0)

	// --- Order 1: Delete first, then insert (transformed) ---
	doc1 := applyOpToContent(initial, deleteOp)
	transformedIns := Transform(insertOp, deleteOp)
	doc1 = applyOpToContent(doc1, transformedIns)

	// Verify the transformed insert position — should move to start of deleted range
	if transformedIns.Position != 5 {
		t.Errorf("transformed insert position: expected 5, got %d", transformedIns.Position)
	}

	// --- Order 2: Insert first, then delete (transformed) ---
	doc2 := applyOpToContent(initial, insertOp)
	transformedDel := Transform(deleteOp, insertOp)
	doc2 = applyOpToContent(doc2, transformedDel)

	// The delete is before the insert (5 < 7), so it stays unchanged
	if transformedDel.Position != 5 || transformedDel.Length != 6 {
		t.Errorf("transformed delete: expected pos=5 len=6, got pos=%d len=%d",
			transformedDel.Position, transformedDel.Length)
	}

	// Both orders must produce the same result.
	expected := "HelloX"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// DELETE + DELETE  —  two deletes with overlapping ranges
// ---------------------------------------------------------------------------

// TestTransformPairDeleteDelete verifies the DELETE+DELETE pair where the
// two delete ranges overlap.
//
// Initial document: "abcdefghij"
// Delete A: position=2, length=5  (deletes [2, 7) = "cdefg")
// Delete B: position=5, length=4  (deletes [5, 9) = "fghi")
//
// Expected: "abj"
//
// The overlap region [5, 7) = "fg" is deleted by both. After both deletes,
// the remaining characters are "ab" (before both ranges) + "j" (after both
// ranges) = "abj".
func TestTransformPairDeleteDelete(t *testing.T) {
	initial := "abcdefghij"

	deleteA := opWithID("del-a", DeleteOperation, 2, "", 5)
	deleteB := opWithID("del-b", DeleteOperation, 5, "", 4)

	// --- Order 1: A first, then B (transformed) ---
	doc1 := applyOpToContent(initial, deleteA)
	transformedB := Transform(deleteB, deleteA)
	doc1 = applyOpToContent(doc1, transformedB)

	// Verify transformed B: B [5,9) overlaps end of A [2,7).
	// remainingLen = 9 - 7 = 2, position = 2 (start of A's range).
	if transformedB.Type != DeleteOperation {
		t.Errorf("transformed B type: expected delete, got %q", transformedB.Type)
	}
	if transformedB.Position != 2 || transformedB.Length != 2 {
		t.Errorf("transformed B: expected pos=2 len=2, got pos=%d len=%d",
			transformedB.Position, transformedB.Length)
	}

	// --- Order 2: B first, then A (transformed) ---
	doc2 := applyOpToContent(initial, deleteB)
	transformedA := Transform(deleteA, deleteB)
	doc2 = applyOpToContent(doc2, transformedA)

	// Verify transformed A: A [2,7) overlaps start of B [5,9).
	// A starts before B, so it keeps [2, 5) → position=2, length=3.
	if transformedA.Type != DeleteOperation {
		t.Errorf("transformed A type: expected delete, got %q", transformedA.Type)
	}
	if transformedA.Position != 2 || transformedA.Length != 3 {
		t.Errorf("transformed A: expected pos=2 len=3, got pos=%d len=%d",
			transformedA.Position, transformedA.Length)
	}

	// Both orders must produce the same result.
	expected := "abj"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// DELETE + DELETE  —  two deletes with a gap (non-overlapping)
// ---------------------------------------------------------------------------

// TestTransformPairDeleteDeleteNonOverlapping verifies the DELETE+DELETE pair
// where the two delete ranges do not overlap.
//
// Initial document: "abcdefghij"
// Delete A: position=0, length=3  (deletes [0, 3) = "abc")
// Delete B: position=5, length=2  (deletes [5, 7) = "fg")
//
// Expected: "dehij"
func TestTransformPairDeleteDeleteNonOverlapping(t *testing.T) {
	initial := "abcdefghij"

	deleteA := opWithID("del-a", DeleteOperation, 0, "", 3)
	deleteB := opWithID("del-b", DeleteOperation, 5, "", 2)

	// --- Order 1: A first, then B (transformed) ---
	doc1 := applyOpToContent(initial, deleteA)
	transformedB := Transform(deleteB, deleteA)
	doc1 = applyOpToContent(doc1, transformedB)

	// B is after A's range. B's position shifts left by A's length (3).
	// 5 - 3 = 2.
	if transformedB.Position != 2 || transformedB.Length != 2 {
		t.Errorf("transformed B: expected pos=2 len=2, got pos=%d len=%d",
			transformedB.Position, transformedB.Length)
	}

	// --- Order 2: B first, then A (transformed) ---
	doc2 := applyOpToContent(initial, deleteB)
	transformedA := Transform(deleteA, deleteB)
	doc2 = applyOpToContent(doc2, transformedA)

	// A is before B's range. A's position is unchanged.
	if transformedA.Position != 0 || transformedA.Length != 3 {
		t.Errorf("transformed A: expected pos=0 len=3, got pos=%d len=%d",
			transformedA.Position, transformedA.Length)
	}

	expected := "dehij"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// DELETE + DELETE  —  one delete completely within the other
// ---------------------------------------------------------------------------

// TestTransformPairDeleteDeleteContained verifies the DELETE+DELETE pair
// where one delete range is completely contained within the other.
//
// Initial document: "abcdefghij"
// Delete A: position=2, length=6  (deletes [2, 8) = "cdefgh")
// Delete B: position=3, length=2  (deletes [3, 5) = "de")
//
// Expected: "abij"
//
// B is completely within A's range, so B becomes a no-op after transformation.
func TestTransformPairDeleteDeleteContained(t *testing.T) {
	initial := "abcdefghij"

	deleteA := opWithID("del-a", DeleteOperation, 2, "", 6)
	deleteB := opWithID("del-b", DeleteOperation, 3, "", 2)

	// --- Order 1: A first, then B (transformed) ---
	doc1 := applyOpToContent(initial, deleteA)
	transformedB := Transform(deleteB, deleteA)
	doc1 = applyOpToContent(doc1, transformedB)

	// B is completely within A's range → should become a no-op.
	if transformedB.Type != NoOpOperation {
		t.Errorf("transformed B type: expected noop, got %q", transformedB.Type)
	}

	// --- Order 2: B first, then A (transformed) ---
	doc2 := applyOpToContent(initial, deleteB)
	transformedA := Transform(deleteA, deleteB)
	doc2 = applyOpToContent(doc2, transformedA)

	// A overlaps B's range. A starts before B, so it keeps [2, 3) → position=2, length=1.
	if transformedA.Type != DeleteOperation {
		t.Errorf("transformed A type: expected delete, got %q", transformedA.Type)
	}
	if transformedA.Position != 2 || transformedA.Length != 1 {
		t.Errorf("transformed A: expected pos=2 len=1, got pos=%d len=%d",
			transformedA.Position, transformedA.Length)
	}

	expected := "abij"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// DELETE vs INSERT  —  delete arrives after an insert has been applied
// ---------------------------------------------------------------------------

// TestTransformPairDeleteInsert verifies the DELETE+INSERT pair where the
// delete arrives after the insert has already been applied.
//
// Initial document: "Hello World"
// Insert: position=5, text=" beautiful"
// Delete: position=5, length=6
//
// Expected: "Hello World"
//
// The insert adds " beautiful" at position 5, making the document
// "Hello beautiful World". The delete at position 5 with length 6 would
// delete " beaut" (the first 6 chars of " beautiful"). After transformation,
// the delete shifts right by len(" beautiful") = 10, so it deletes at
// position 15, removing " World". Result: "Hello beautiful".
func TestTransformPairDeleteInsert(t *testing.T) {
	initial := "Hello World"

	insertOp := opWithID("ins-1", InsertOperation, 5, " beautiful", 0)
	deleteOp := opWithID("del-1", DeleteOperation, 5, "", 6)

	// --- Order 1: Insert first, then delete (transformed) ---
	doc1 := applyOpToContent(initial, insertOp)
	transformedDel := Transform(deleteOp, insertOp)
	doc1 = applyOpToContent(doc1, transformedDel)

	// The delete at position 5 >= insert position 5, so it shifts right by 10.
	// 5 + 10 = 15.
	if transformedDel.Position != 15 || transformedDel.Length != 6 {
		t.Errorf("transformed delete: expected pos=15 len=6, got pos=%d len=%d",
			transformedDel.Position, transformedDel.Length)
	}

	// --- Order 2: Delete first, then insert (transformed) ---
	doc2 := applyOpToContent(initial, deleteOp)
	transformedIns := Transform(insertOp, deleteOp)
	doc2 = applyOpToContent(doc2, transformedIns)

	// The insert at position 5 is within the deleted range [5, 11).
	// It moves to the start of the deleted range (position 5).
	if transformedIns.Position != 5 {
		t.Errorf("transformed insert position: expected 5, got %d", transformedIns.Position)
	}

	// Both orders must produce the same result.
	expected := "Hello beautiful"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// INSERT + INSERT  —  inserts at different positions
// ---------------------------------------------------------------------------

// TestTransformPairInsertInsertDifferentPositions verifies the INSERT+INSERT
// pair where the two inserts target different positions.
//
// Initial document: "Hello"
// A: insert("A", 2)
// B: insert("B", 4)
//
// Expected: "HeAlBlo"
func TestTransformPairInsertInsertDifferentPositions(t *testing.T) {
	initial := "Hello"

	opA := opWithID("aaa", InsertOperation, 2, "A", 0)
	opB := opWithID("bbb", InsertOperation, 4, "B", 0)

	// --- Order 1: A first, then B (transformed) ---
	doc1 := applyOpToContent(initial, opA)
	transformedB := Transform(opB, opA)
	doc1 = applyOpToContent(doc1, transformedB)

	// B at position 4 > A at position 2, so B shifts right by len("A") = 1.
	// 4 + 1 = 5.
	if transformedB.Position != 5 {
		t.Errorf("transformed B position: expected 5, got %d", transformedB.Position)
	}

	// --- Order 2: B first, then A (transformed) ---
	doc2 := applyOpToContent(initial, opB)
	transformedA := Transform(opA, opB)
	doc2 = applyOpToContent(doc2, transformedA)

	// A at position 2 < B at position 4, so A's position is unchanged.
	if transformedA.Position != 2 {
		t.Errorf("transformed A position: expected 2, got %d", transformedA.Position)
	}

	expected := "HeAlBlo"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// INSERT + DELETE  —  insert before the deleted range
// ---------------------------------------------------------------------------

// TestTransformPairInsertDeleteBefore verifies the INSERT+DELETE pair where
// the insert is before the deleted range.
//
// Initial document: "Hello World"
// Delete: position=5, length=6  (removes " World")
// Insert: position=2, text="X"
//
// Expected: "HeXllo"
func TestTransformPairInsertDeleteBefore(t *testing.T) {
	initial := "Hello World"

	deleteOp := opWithID("del-1", DeleteOperation, 5, "", 6)
	insertOp := opWithID("ins-1", InsertOperation, 2, "X", 0)

	// --- Order 1: Delete first, then insert (transformed) ---
	doc1 := applyOpToContent(initial, deleteOp)
	transformedIns := Transform(insertOp, deleteOp)
	doc1 = applyOpToContent(doc1, transformedIns)

	// Insert at position 2 < delete position 5, so position is unchanged.
	if transformedIns.Position != 2 {
		t.Errorf("transformed insert position: expected 2, got %d", transformedIns.Position)
	}

	// --- Order 2: Insert first, then delete (transformed) ---
	doc2 := applyOpToContent(initial, insertOp)
	transformedDel := Transform(deleteOp, insertOp)
	doc2 = applyOpToContent(doc2, transformedDel)

	// Delete at position 5 >= insert position 2, so delete shifts right by 1.
	// 5 + 1 = 6.
	if transformedDel.Position != 6 || transformedDel.Length != 6 {
		t.Errorf("transformed delete: expected pos=6 len=6, got pos=%d len=%d",
			transformedDel.Position, transformedDel.Length)
	}

	expected := "HeXllo"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// INSERT + DELETE  —  insert after the deleted range
// ---------------------------------------------------------------------------

// TestTransformPairInsertDeleteAfter verifies the INSERT+DELETE pair where
// the insert is after the deleted range.
//
// Initial document: "Hello World"
// Delete: position=0, length=5  (removes "Hello")
// Insert: position=8, text="X"
//
// Expected: " XWorld"
func TestTransformPairInsertDeleteAfter(t *testing.T) {
	initial := "Hello World"

	deleteOp := opWithID("del-1", DeleteOperation, 0, "", 5)
	insertOp := opWithID("ins-1", InsertOperation, 8, "X", 0)

	// --- Order 1: Delete first, then insert (transformed) ---
	doc1 := applyOpToContent(initial, deleteOp)
	transformedIns := Transform(insertOp, deleteOp)
	doc1 = applyOpToContent(doc1, transformedIns)

	// Insert at position 8 >= delete end (0+5=5), so shift left by 5.
	// 8 - 5 = 3.
	if transformedIns.Position != 3 {
		t.Errorf("transformed insert position: expected 3, got %d", transformedIns.Position)
	}

	// --- Order 2: Insert first, then delete (transformed) ---
	doc2 := applyOpToContent(initial, insertOp)
	transformedDel := Transform(deleteOp, insertOp)
	doc2 = applyOpToContent(doc2, transformedDel)

	// Delete at position 0 < insert position 8, so delete is unchanged.
	if transformedDel.Position != 0 || transformedDel.Length != 5 {
		t.Errorf("transformed delete: expected pos=0 len=5, got pos=%d len=%d",
			transformedDel.Position, transformedDel.Length)
	}

	expected := " XWorld"
	if doc1 != expected {
		t.Errorf("order 1: expected %q, got %q", expected, doc1)
	}
	if doc2 != expected {
		t.Errorf("order 2: expected %q, got %q", expected, doc2)
	}
}

// ---------------------------------------------------------------------------
// CONVERGENCE TESTS — the core convergence property
// ---------------------------------------------------------------------------

// checkConvergence is a helper that verifies the core convergence property for
// two concurrent operations:
//
//	Apply(A, Transform(B, A)) == Apply(B, Transform(A, B))
//
// It applies A first then transformed-B (order A→B), and B first then
// transformed-A (order B→A), and asserts both produce the same document.
// If expected is non-empty, it also asserts the final document matches.
func checkConvergence(t *testing.T, initial string, opA, opB Operation, expected string) {
	t.Helper()

	// --- Order 1: A → B ---
	// Apply A first, then apply B transformed against A.
	doc1 := applyOpToContent(initial, opA)
	transformedB := Transform(opB, opA)
	doc1 = applyOpToContent(doc1, transformedB)

	// --- Order 2: B → A ---
	// Apply B first, then apply A transformed against B.
	doc2 := applyOpToContent(initial, opB)
	transformedA := Transform(opA, opB)
	doc2 = applyOpToContent(doc2, transformedA)

	// Convergence: both orders must produce the same final document.
	if doc1 != doc2 {
		t.Errorf("convergence failed: order A→B produced %q, order B→A produced %q", doc1, doc2)
	}

	if expected != "" && doc1 != expected {
		t.Errorf("expected %q, got %q (order A→B) and %q (order B→A)", expected, doc1, doc2)
	}
}

// TestConvergenceInsertInsertSamePosition is the most important test. It
// verifies the core convergence property for two concurrent inserts at the
// same position.
//
// Initial document: "Hello"
// A = INSERT(" A", position=5)
// B = INSERT(" B", position=5)
//
// Because both inserts target the same position, the tie-breaker (operation ID)
// determines ordering. With A.ID ("aaa") < B.ID ("bbb"), A is applied first
// in both orders, yielding "Hello A B".
//
// Conceptually:
//
//	Transform(B, A) and Transform(A, B) must ensure:
//	Apply(A, Transform(B, A)) == Apply(B, Transform(A, B))
func TestConvergenceInsertInsertSamePosition(t *testing.T) {
	initial := "Hello"

	// A = INSERT(" A", position=5)
	// B = INSERT(" B", position=5)
	opA := opWithID("aaa", InsertOperation, 5, " A", 0)
	opB := opWithID("bbb", InsertOperation, 5, " B", 0)

	checkConvergence(t, initial, opA, opB, "Hello A B")
}

// TestConvergenceInsertInsertSamePositionReverseIDs tests the same scenario
// with reversed IDs to confirm convergence holds regardless of which
// operation wins the tie-break.
//
// With A.ID ("bbb") > B.ID ("aaa"), B wins the tie-break and goes first,
// yielding "Hello B A".
func TestConvergenceInsertInsertSamePositionReverseIDs(t *testing.T) {
	initial := "Hello"

	opA := opWithID("bbb", InsertOperation, 5, " A", 0)
	opB := opWithID("aaa", InsertOperation, 5, " B", 0)

	checkConvergence(t, initial, opA, opB, "Hello B A")
}

// TestConvergenceInsertInsertDifferentText verifies convergence with
// different text lengths at the same position, confirming the property is
// not specific to the exact " A"/" B" text.
//
// Initial document: "Hello"
// A = INSERT(" World", position=5)
// B = INSERT("!", position=5)
//
// With A.ID ("aaa") < B.ID ("bbb"), A goes first: "Hello World!".
func TestConvergenceInsertInsertDifferentText(t *testing.T) {
	initial := "Hello"

	opA := opWithID("aaa", InsertOperation, 5, " World", 0)
	opB := opWithID("bbb", InsertOperation, 5, "!", 0)

	checkConvergence(t, initial, opA, opB, "Hello World!")
}

// TestConvergenceInsertInsertDifferentPositions verifies convergence when
// the two inserts target different positions.
//
// Initial document: "Hello"
// A = INSERT("A", position=2)
// B = INSERT("B", position=4)
//
// Order A→B: apply A at pos 2 → "HeAllo", transform B at pos 4 → pos 5,
// apply B → "HeAllBo"
// Order B→A: apply B at pos 4 → "HellBo", transform A at pos 2 → pos 2,
// apply A → "HeAllBo"
//
// Expected: "HeAllBo"
func TestConvergenceInsertInsertDifferentPositions(t *testing.T) {
	initial := "Hello"

	opA := opWithID("aaa", InsertOperation, 2, "A", 0)
	opB := opWithID("bbb", InsertOperation, 4, "B", 0)

	checkConvergence(t, initial, opA, opB, "HeAllBo")

}

// TestConvergenceInsertDelete verifies convergence across operation types
// (INSERT + DELETE).
//
// Initial document: "Hello World"
// A = DELETE(position=5, length=6)  (removes " World")
// B = INSERT("X", position=11)
//
// Expected: "HelloX"
func TestConvergenceInsertDelete(t *testing.T) {
	initial := "Hello World"

	opA := opWithID("del-1", DeleteOperation, 5, "", 6)
	opB := opWithID("ins-1", InsertOperation, 11, "X", 0)

	checkConvergence(t, initial, opA, opB, "HelloX")
}

// TestConvergenceDeleteDelete verifies convergence for two deletes with
// overlapping ranges.
//
// Initial document: "abcdefghij"
// A = DELETE(position=2, length=5)  (deletes [2, 7) = "cdefg")
// B = DELETE(position=5, length=4)  (deletes [5, 9) = "fghi")
//
// Expected: "abj"
func TestConvergenceDeleteDelete(t *testing.T) {
	initial := "abcdefghij"

	opA := opWithID("del-a", DeleteOperation, 2, "", 5)
	opB := opWithID("del-b", DeleteOperation, 5, "", 4)

	checkConvergence(t, initial, opA, opB, "abj")
}


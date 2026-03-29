package websocket

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Validate tests
// ---------------------------------------------------------------------------

// TestValidateValidInsert verifies that a valid insert operation passes validation.
func TestValidateValidInsert(t *testing.T) {
	op := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}
	if err := op.Validate(0); err != nil {
		t.Errorf("expected no error for valid insert, got: %v", err)
	}
}

// TestValidateInsertAtEnd verifies that inserting at the end of the content
// (position == contentLength) is valid.
//
// Content: "Hello" (length 5)
// Insert at position 5 → valid (inserts after the last character)
func TestValidateInsertAtEnd(t *testing.T) {
	op := Operation{
		Type:     InsertOperation,
		Position: 5,
		Text:     " World",
	}
	if err := op.Validate(5); err != nil {
		t.Errorf("expected no error for insert at end, got: %v", err)
	}
}

// TestValidateInsertEmptyText verifies that an insert with empty text is rejected.
//
// Content: "Hello"
// Insert "" at position 2 → rejected
func TestValidateInsertEmptyText(t *testing.T) {
	op := Operation{
		Type:     InsertOperation,
		Position: 2,
		Text:     "",
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected error about empty text, got: %v", err)
	}
}

// TestValidateInsertPositionOutOfBounds verifies that an insert with a position
// beyond the content length is rejected with "position out of bounds".
//
// Content: "Hello" (length 5)
// Insert "X" at position 100 → rejected: "position out of bounds"
func TestValidateInsertPositionOutOfBounds(t *testing.T) {
	op := Operation{
		Type:     InsertOperation,
		Position: 100,
		Text:     "X",
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for position out of bounds, got nil")
	}
	if !strings.Contains(err.Error(), "position out of bounds") {
		t.Errorf("expected 'position out of bounds' error, got: %v", err)
	}
}

// TestValidateInsertNegativePosition verifies that an insert with a negative
// position is rejected.
//
// Content: "Hello"
// Insert "X" at position -1 → rejected
func TestValidateInsertNegativePosition(t *testing.T) {
	op := Operation{
		Type:     InsertOperation,
		Position: -1,
		Text:     "X",
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for negative position, got nil")
	}
	if !strings.Contains(err.Error(), "position out of bounds") {
		t.Errorf("expected 'position out of bounds' error, got: %v", err)
	}
}

// TestValidateInsertPositionOnePastEnd verifies that an insert with a position
// one past the end is rejected.
//
// Content: "Hello" (length 5)
// Insert "X" at position 6 → rejected (valid range is 0-5)
func TestValidateInsertPositionOnePastEnd(t *testing.T) {
	op := Operation{
		Type:     InsertOperation,
		Position: 6,
		Text:     "X",
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for position past end, got nil")
	}
}

// TestValidateInsertOnEmptyContent verifies that inserting into empty content
// at position 0 is valid.
func TestValidateInsertOnEmptyContent(t *testing.T) {
	op := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}
	if err := op.Validate(0); err != nil {
		t.Errorf("expected no error for insert on empty content, got: %v", err)
	}
}

// TestValidateValidDelete verifies that a valid delete operation passes validation.
func TestValidateValidDelete(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 0,
		Length:   5,
	}
	if err := op.Validate(10); err != nil {
		t.Errorf("expected no error for valid delete, got: %v", err)
	}
}

// TestValidateDeletePositionOutOfBounds verifies that a delete with a position
// beyond the content length is rejected.
func TestValidateDeletePositionOutOfBounds(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 10,
		Length:   1,
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for delete position out of bounds, got nil")
	}
}

// TestValidateDeleteNegativePosition verifies that a delete with a negative
// position is rejected.
func TestValidateDeleteNegativePosition(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: -1,
		Length:   1,
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for delete negative position, got nil")
	}
}

// TestValidateDeleteOnEmptyContent verifies that a delete on empty content
// is rejected (there is nothing to delete).
func TestValidateDeleteOnEmptyContent(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 0,
		Length:   1,
	}
	err := op.Validate(0)
	if err == nil {
		t.Fatal("expected error for delete on empty content, got nil")
	}
}

// TestValidateDeleteZeroLength verifies that a delete with length 0 is rejected.
func TestValidateDeleteZeroLength(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 0,
		Length:   0,
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for delete with zero length, got nil")
	}
	if !strings.Contains(err.Error(), "length") {
		t.Errorf("expected error about length, got: %v", err)
	}
}

// TestValidateDeleteNegativeLength verifies that a delete with a negative
// length is rejected.
//
// Content: "Hello" (length 5)
// Delete -1 characters at position 2 → rejected: "length must be positive"
func TestValidateDeleteNegativeLength(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 2,
		Length:   -1,
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for delete with negative length, got nil")
	}
	if !strings.Contains(err.Error(), "length") {
		t.Errorf("expected error about length, got: %v", err)
	}
}

// TestValidateDeleteLengthExceedsContent verifies that a delete where
// position + length exceeds the content length is rejected.
//
// This is the exact example from the task specification:
//
//	Document: "Hello" (length 5)
//	Delete: position = 3, length = 10
//	Reject because: 3 + 10 > 5
func TestValidateDeleteLengthExceedsContent(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 3,
		Length:   10,
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for delete range exceeding content length, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected error about exceeding content length, got: %v", err)
	}
}

// TestValidateDeleteRangeExceedsContent verifies rejection when position +
// length is just one past the content length.
//
// Content: "Hello" (length 5)
// Delete 6 characters at position 0 → rejected: 0 + 6 > 5
func TestValidateDeleteRangeExceedsContent(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 0,
		Length:   6,
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for delete range exceeding content length, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected error about exceeding content length, got: %v", err)
	}
}

// TestValidateDeleteExactlyToContentEnd verifies that a delete where
// position + length exactly equals the content length is valid.
//
// Content: "Hello" (length 5)
// Delete 5 characters at position 0 → valid: 0 + 5 == 5 (not > 5)
// Result: "" (entire content deleted)
func TestValidateDeleteExactlyToContentEnd(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 0,
		Length:   5,
	}
	if err := op.Validate(5); err != nil {
		t.Errorf("expected no error for delete exactly to content end, got: %v", err)
	}
}

// TestValidateDeletePartialRangeValid verifies that a delete where
// position + length is within the content length is valid.
//
// Content: "Hello" (length 5)
// Delete 2 characters at position 3 → valid: 3 + 2 == 5 (not > 5)
// Result: "Hel"
func TestValidateDeletePartialRangeValid(t *testing.T) {
	op := Operation{
		Type:     DeleteOperation,
		Position: 3,
		Length:   2,
	}
	if err := op.Validate(5); err != nil {
		t.Errorf("expected no error for valid partial delete range, got: %v", err)
	}
}

// TestValidateInvalidOperationType verifies that an operation with an
// unrecognized type is rejected.
//
// Operation: {"type": "update"} → rejected: "invalid operation type"
func TestValidateInvalidOperationType(t *testing.T) {
	op := Operation{
		Type:     OperationType("update"),
		Position: 0,
		Text:     "Hello",
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for invalid operation type, got nil")
	}
	if !strings.Contains(err.Error(), "invalid operation type") {
		t.Errorf("expected 'invalid operation type' error, got: %v", err)
	}
}

// TestValidateEmptyOperationType verifies that an operation with an empty
// type is rejected.
func TestValidateEmptyOperationType(t *testing.T) {
	op := Operation{
		Type:     "",
		Position: 0,
		Text:     "Hello",
	}
	err := op.Validate(5)
	if err == nil {
		t.Fatal("expected error for empty operation type, got nil")
	}
}

// TestValidateAllValidPositionsForHello verifies that every valid insert position
// for the document "Hello" (length 5) passes validation.
//
// Valid positions: 0, 1, 2, 3, 4, 5
func TestValidateAllValidPositionsForHello(t *testing.T) {
	for pos := 0; pos <= 5; pos++ {
		op := Operation{
			Type:     InsertOperation,
			Position: pos,
			Text:     "X",
		}
		if err := op.Validate(5); err != nil {
			t.Errorf("position %d should be valid for content length 5, got: %v", pos, err)
		}
	}
}

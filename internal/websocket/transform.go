package websocket

// Transform adjusts the incoming operation so it can be applied after the
// applied operation has already been applied to the document. This is the
// core of operational transformation for collaborative editing.
//
// The function supports all four conflict combinations:
//   - INSERT vs INSERT
//   - INSERT vs DELETE
//   - DELETE vs INSERT
//   - DELETE vs DELETE
//
// Both operations are assumed to be valid and originally based on the same
// document state. The returned operation preserves the incoming operation's
// ID, ClientID, and BaseVersion but with adjusted Position (and Length for
// deletes) so that applying it after the applied operation produces the same
// final document state as applying the operations in original order.
func Transform(incoming, applied Operation) Operation {
	result := incoming
	var isNoOp bool

	switch incoming.Type {
	case InsertOperation:
		switch applied.Type {
		case InsertOperation:
			result.Position = transformInsertAfterInsert(incoming, applied)
		case DeleteOperation:
			result.Position = transformInsertAfterDelete(incoming.Position, applied)
		}
	case DeleteOperation:
		switch applied.Type {
		case InsertOperation:
			result.Position, result.Length = transformDeleteAfterInsert(incoming.Position, incoming.Length, applied)
		case DeleteOperation:
			result.Position, result.Length, isNoOp = transformDeleteAfterDelete(incoming.Position, incoming.Length, applied)
			if isNoOp {
				result = TransformToNoop(incoming)
			}
		}
	}

	return result
}

// TransformToNoop returns a no-op version of the given operation.
// A no-op has NoOpOperation type and zeroed positional fields.
func TransformToNoop(op Operation) Operation {
	return Operation{
		ID:          op.ID,
		ClientID:    op.ClientID,
		Type:        NoOpOperation,
		Position:    0,
		Text:        "",
		Length:      0,
		BaseVersion: op.BaseVersion,
	}
}

// transformInsertAfterInsert handles an insert arriving after another insert
// has already been applied.
//
// Rules:
//   - If incoming position < applied position: the incoming insert is before
//     the applied text, so its position is unchanged.
//   - If incoming position > applied position: the applied text shifts all
//     subsequent positions right, so add len(applied.Text) to the position.
//   - If incoming position == applied position: deterministic tie-breaking
//     by operation ID. The operation with the smaller ID is applied first.
//     Since the applied operation is already in the document, if the incoming
//     operation has the smaller ID, it should go at the applied position
//     (before the applied text). Otherwise, it goes after the applied text.
func transformInsertAfterInsert(incoming, applied Operation) int {
	if incoming.Position < applied.Position {
		return incoming.Position
	}
	if incoming.Position > applied.Position {
		return incoming.Position + len(applied.Text)
	}
	// Positions are equal: use deterministic tie-breaking by ID
	if incoming.ID < applied.ID {
		return applied.Position
	}
	return applied.Position + len(applied.Text)
}

// transformInsertAfterDelete handles an insert arriving after a delete has
// already been applied.
//
// Rules:
//   - If incoming position < applied position: the insert is before the
//     deleted range, so its position is unchanged.
//   - If incoming position >= applied.Position + applied.Length: the insert
//     is after the deleted range, so shift left by applied.Length.
//   - If incoming position falls within [applied.Position,
//     applied.Position + applied.Length): the original text was deleted, so
//     the insert moves to the start of the deleted range.
func transformInsertAfterDelete(incomingPos int, applied Operation) int {
	if incomingPos < applied.Position {
		return incomingPos
	}
	if incomingPos >= applied.Position+applied.Length {
		return incomingPos - applied.Length
	}
	return applied.Position
}

// transformDeleteAfterInsert handles a delete arriving after an insert has
// already been applied.
//
// Rules:
//   - If incoming position < applied position: the delete is before the
//     inserted text, so both position and length are unchanged.
//   - If incoming position >= applied position: the delete is after the
//     inserted text, so shift the position right by len(applied.Text).
func transformDeleteAfterInsert(incomingPos, incomingLen int, applied Operation) (int, int) {
	if incomingPos < applied.Position {
		return incomingPos, incomingLen
	}
	return incomingPos + len(applied.Text), incomingLen
}

// transformDeleteAfterDelete handles a delete arriving after another delete
// has already been applied.
//
// The applied delete has removed [applied.Position, applied.Position +
// applied.Length). The incoming delete targeted [incomingPos, incomingPos +
// incomingLen). We compute the remaining range that still needs to be removed.
func transformDeleteAfterDelete(incomingPos, incomingLen int, applied Operation) (int, int, bool) {
	appliedEnd := applied.Position + applied.Length
	incomingEnd := incomingPos + incomingLen

	// Case 1: incoming is completely before the applied delete
	if incomingEnd <= applied.Position {
		return incomingPos, incomingLen, false
	}

	// Case 2: incoming is completely after the applied delete
	if incomingPos >= appliedEnd {
		return incomingPos - applied.Length, incomingLen, false
	}

	// Cases 3-5: overlaps of various kinds
	if incomingPos < applied.Position {
		// Case 3: overlaps start of applied delete
		return incomingPos, applied.Position - incomingPos, false
	}

	// incomingPos >= applied.Position
	remainingLen := incomingEnd - appliedEnd
	if remainingLen <= 0 {
		// Case 4: completely within applied delete - return NO-OP
		return 0, 0, true
	}
	// Case 5: overlaps end of applied delete
	return applied.Position, remainingLen, false
}

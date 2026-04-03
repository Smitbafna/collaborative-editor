package websocket

import (
	"math/rand"
	"testing"
)

// ---------------------------------------------------------------------------
// Property-style (randomized) convergence tests
//
// For two randomly generated operations A and B, we verify that:
//
//	Apply(A, Transform(B, A)) == Apply(B, Transform(A, B))
//
// This is much stronger than manually testing only a few examples.
// ---------------------------------------------------------------------------

// randomString generates a random string of the given length.
// It uses lowercase letters to keep things simple but varied.
func randomString(rng *rand.Rand, length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	if length <= 0 {
		return ""
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

// randomOperation generates a random valid operation for a document of the given length.
// It returns either an INSERT or DELETE operation with a random ID.
func randomOperation(rng *rand.Rand, docLen int, id string) Operation {
	if docLen == 0 {
		// Only inserts are possible on an empty document
		pos := 0
		textLen := rng.Intn(5) + 1 // 1-5 characters
		return opWithID(id, InsertOperation, pos, randomString(rng, textLen), 0)
	}

	// Decide whether to insert or delete (roughly 50/50)
	if rng.Intn(2) == 0 {
		// INSERT: position anywhere from 0 to docLen, text of length 1-5
		pos := rng.Intn(docLen + 1)
		textLen := rng.Intn(5) + 1
		return opWithID(id, InsertOperation, pos, randomString(rng, textLen), 0)
	}

	// DELETE: position anywhere from 0 to docLen-1, length from 1 to remaining
	pos := rng.Intn(docLen)
	maxLen := docLen - pos
	if maxLen <= 0 {
		maxLen = 1
	}
	length := rng.Intn(maxLen) + 1
	return opWithID(id, DeleteOperation, pos, "", length)
}

// TestTransformProperty_RandomOperations generates random documents and random
// operation pairs, then verifies that the convergence property holds for every
// pair. This is a property-based test that exercises the full range of
// conflict combinations (INSERT+INSERT, INSERT+DELETE, DELETE+INSERT,
// DELETE+DELETE) including edge cases at boundaries.
func TestTransformProperty_RandomOperations(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed for reproducibility

	const iterations = 10000
	const maxInitialLen = 20

	chars := 0
	opsGen := 0
	for i := 0; i < iterations; i++ {
		// Generate a random initial document (0 to maxInitialLen characters)
		initialLen := rng.Intn(maxInitialLen + 1)
		initial := randomString(rng, initialLen)
		chars += initialLen

		// Generate two random operations that are valid for this document
		opA := randomOperation(rng, len(initial), "A")
		opB := randomOperation(rng, len(initial), "B")
		opsGen += 2

		// Verify convergence
		doc1 := applyOpToContent(initial, opA)
		transformedB := Transform(opB, opA)
		doc1 = applyOpToContent(doc1, transformedB)

		doc2 := applyOpToContent(initial, opB)
		transformedA := Transform(opA, opB)
		doc2 = applyOpToContent(doc2, transformedA)

		if doc1 != doc2 {
			t.Errorf("CONVERGENCE FAILED (iteration %d)\n"+
				"  initial:   %q (len=%d)\n"+
				"  opA:       {type=%s pos=%d text=%q len=%d id=%s}\n"+
				"  opB:       {type=%s pos=%d text=%q len=%d id=%s}\n"+
				"  order A→B: %q\n"+
				"  order B→A: %q\n",
				i,
				initial, len(initial),
				opA.Type, opA.Position, opA.Text, opA.Length, opA.ID,
				opB.Type, opB.Position, opB.Text, opB.Length, opB.ID,
				doc1, doc2)
			return // stop on first failure to avoid spam
		}
	}

	t.Logf("passed %d convergence checks on %d initial document chars (%d ops generated)",
		iterations, chars, opsGen)
}

// TestTransformProperty_EdgeCases generates operations at known edge positions
// (0, docLen-1, docLen, middle) and verifies convergence. This complements the
// purely random test by ensuring edge positions are thoroughly exercised.
func TestTransformProperty_EdgeCases(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	const iterations = 5000

	for i := 0; i < iterations; i++ {
		// Random document length (0 to 15)
		initialLen := rng.Intn(16)
		initial := randomString(rng, initialLen)

		// Pick edge positions for operations
		edgePositions := []int{0}
		if initialLen > 0 {
			edgePositions = append(edgePositions, initialLen-1)
		}
		if initialLen > 1 {
			edgePositions = append(edgePositions, initialLen/2)
		}
		edgePositions = append(edgePositions, initialLen)

		// Pick two random edge positions
		if len(edgePositions) < 2 {
			continue
		}
		posA := edgePositions[rng.Intn(len(edgePositions))]
		posB := edgePositions[rng.Intn(len(edgePositions))]

		// Generate operation A
		var opA Operation
		if initialLen == 0 || rng.Intn(2) == 0 {
			textLen := rng.Intn(5) + 1
			opA = opWithID("A", InsertOperation, posA, randomString(rng, textLen), 0)
		} else {
			maxLen := initialLen - posA
			if maxLen <= 0 {
				maxLen = 1
			}
			length := rng.Intn(maxLen) + 1
			opA = opWithID("A", DeleteOperation, posA, "", length)
		}

		// Generate operation B
		var opB Operation
		if initialLen == 0 || rng.Intn(2) == 0 {
			textLen := rng.Intn(5) + 1
			opB = opWithID("B", InsertOperation, posB, randomString(rng, textLen), 0)
		} else {
			maxLen := initialLen - posB
			if maxLen <= 0 {
				maxLen = 1
			}
			length := rng.Intn(maxLen) + 1
			opB = opWithID("B", DeleteOperation, posB, "", length)
		}

		// Verify convergence
		doc1 := applyOpToContent(initial, opA)
		transformedB := Transform(opB, opA)
		doc1 = applyOpToContent(doc1, transformedB)

		doc2 := applyOpToContent(initial, opB)
		transformedA := Transform(opA, opB)
		doc2 = applyOpToContent(doc2, transformedA)

		if doc1 != doc2 {
			t.Errorf("CONVERGENCE FAILED (edge case iteration %d)\n"+
				"  initial:   %q (len=%d)\n"+
				"  opA:       {type=%s pos=%d text=%q len=%d id=%s}\n"+
				"  opB:       {type=%s pos=%d text=%q len=%d id=%s}\n"+
				"  order A→B: %q\n"+
				"  order B→A: %q\n",
				i,
				initial, len(initial),
				opA.Type, opA.Position, opA.Text, opA.Length, opA.ID,
				opB.Type, opB.Position, opB.Text, opB.Length, opB.ID,
				doc1, doc2)
			return
		}
	}
}

// TestTransformProperty_SameIDCornerCase verifies that operations with equal
// IDs do not cause issues. While in practice IDs should be unique, the
// tie-breaking logic could still receive equal IDs.
func TestTransformProperty_SameIDCornerCase(t *testing.T) {
	rng := rand.New(rand.NewSource(456))
	const iterations = 500

	for i := 0; i < iterations; i++ {
		initialLen := rng.Intn(20)
		initial := randomString(rng, initialLen)

		// Both operations have the SAME ID
		opA := randomOperation(rng, len(initial), "same-id")
		opB := randomOperation(rng, len(initial), "same-id")

		doc1 := applyOpToContent(initial, opA)
		transformedB := Transform(opB, opA)
		doc1 = applyOpToContent(doc1, transformedB)

		doc2 := applyOpToContent(initial, opB)
		transformedA := Transform(opA, opB)
		doc2 = applyOpToContent(doc2, transformedA)

		if doc1 != doc2 {
			t.Errorf("CONVERGENCE FAILED (same-ID iteration %d)\n"+
				"  initial:   %q (len=%d)\n"+
				"  opA:       {type=%s pos=%d text=%q len=%d id=%s}\n"+
				"  opB:       {type=%s pos=%d text=%q len=%d id=%s}\n"+
				"  order A→B: %q\n"+
				"  order B→A: %q\n",
				i,
				initial, len(initial),
				opA.Type, opA.Position, opA.Text, opA.Length, opA.ID,
				opB.Type, opB.Position, opB.Text, opB.Length, opB.ID,
				doc1, doc2)
			return
		}
	}
}

// TestTransformProperty_LongTextInserts uses longer insert text (up to 20
// characters) to test that position shifts are computed correctly for
// non-trivial text lengths.
func TestTransformProperty_LongTextInserts(t *testing.T) {
	rng := rand.New(rand.NewSource(789))
	const iterations = 2000

	for i := 0; i < iterations; i++ {
		initialLen := rng.Intn(15)
		initial := randomString(rng, initialLen)

		// Generate operation A: insert with text length 1-20
		posA := rng.Intn(initialLen + 1)
		textLenA := rng.Intn(20) + 1
		opA := opWithID("A", InsertOperation, posA, randomString(rng, textLenA), 0)

		// Generate operation B: could be either type
		var opB Operation
		if rng.Intn(2) == 0 {
			posB := rng.Intn(initialLen + 1)
			textLenB := rng.Intn(20) + 1
			opB = opWithID("B", InsertOperation, posB, randomString(rng, textLenB), 0)
		} else if initialLen > 0 {
			posB := rng.Intn(initialLen)
			maxLen := initialLen - posB
			length := rng.Intn(maxLen) + 1
			opB = opWithID("B", DeleteOperation, posB, "", length)
		} else {
			// Can only insert on empty doc
			posB := 0
			textLenB := rng.Intn(20) + 1
			opB = opWithID("B", InsertOperation, posB, randomString(rng, textLenB), 0)
		}

		doc1 := applyOpToContent(initial, opA)
		transformedB := Transform(opB, opA)
		doc1 = applyOpToContent(doc1, transformedB)

		doc2 := applyOpToContent(initial, opB)
		transformedA := Transform(opA, opB)
		doc2 = applyOpToContent(doc2, transformedA)

		if doc1 != doc2 {
			t.Errorf("CONVERGENCE FAILED (long-text iteration %d)\n"+
				"  initial:   %q (len=%d)\n"+
				"  opA:       {type=%s pos=%d text=%q len=%d id=%s}\n"+
				"  opB:       {type=%s pos=%d text=%q len=%d id=%s}\n"+
				"  order A→B: %q\n"+
				"  order B→A: %q\n",
				i,
				initial, len(initial),
				opA.Type, opA.Position, opA.Text, opA.Length, opA.ID,
				opB.Type, opB.Position, opB.Text, opB.Length, opB.ID,
				doc1, doc2)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmark: measure throughput of the convergence check
// ---------------------------------------------------------------------------

func BenchmarkTransformConvergence(b *testing.B) {
	rng := rand.New(rand.NewSource(999))

	// Pre-generate operations to avoid benchmark overhead from random generation
	type testCase struct {
		initial string
		opA     Operation
		opB     Operation
	}
	const numCases = 1000
	cases := make([]testCase, numCases)
	for i := 0; i < numCases; i++ {
		initialLen := rng.Intn(30)
		initial := randomString(rng, initialLen)
		opA := randomOperation(rng, len(initial), "bench-a")
		opB := randomOperation(rng, len(initial), "bench-b")
		cases[i] = testCase{initial, opA, opB}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc := cases[i%numCases]
		doc1 := applyOpToContent(tc.initial, tc.opA)
		transformedB := Transform(tc.opB, tc.opA)
		doc1 = applyOpToContent(doc1, transformedB)

		doc2 := applyOpToContent(tc.initial, tc.opB)
		transformedA := Transform(tc.opA, tc.opB)
		doc2 = applyOpToContent(doc2, transformedA)

		if doc1 != doc2 {
			b.Fatal("convergence failed in benchmark")
		}
	}
}
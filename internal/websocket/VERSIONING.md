# Document Versioning Rules

## Overview

This document defines the versioning rules for collaborative document operations in the WebSocket-based real-time editor. The versioning system provides:

1. **Ordering** — A total ordering of all document states
2. **Conflict detection** — Clients can detect when their view is stale
3. **Idempotency** — Safe retries without creating duplicate versions
4. **Historical replay** — Clients can reconstruct any version from history

---

## State Machine

A document progresses through versions via operation application:

```
Initial Document
      │
      │  Version = 0
      │
      ▼
  [Operation arrives]
      │
      ├── Invalid ──► (version unchanged)
      │
      ├── Duplicate ──► (version unchanged)
      │
      └── Valid ──► apply ──► Version + 1
```

---

## Rule 1: Initial State

On creation, every document starts at **Version 0**.

```go
// NewRoom creates a room with the initial version
func NewRoom(id string) *Room {
    return &Room{
        Version: 0,  // <-- initial version
        // ...
    }
}
```

**Implications:**
- The first valid operation creates **Version 1**.
- Version 0 represents an empty, unmodified document.
- Clients must send `BaseVersion: 0` on their first operation.

---

## Rule 2: Valid Operations Increment Version

Every **valid, accepted operation** creates exactly one new version.

```
v5  ──valid operation──►  v6
```

The increment is atomic and occurs only after the operation has been successfully applied to the document content.

```go
// ApplyOperation atomically applies an operation and increments version
func (r *Room) ApplyOperation(op Operation) (string, int) {
    // ...
    r.Content = apply(op)
    r.Version++           // <-- increment
    r.History = append(...) // <-- record
    return r.Content, r.Version
}
```

**Successful operation example:**
- Server Version: 5, Content: "Hello World"
- Operation: `INSERT("!", position=11)`
- Result: Version 6, Content: "Hello World!"

---

## Rule 3: Invalid Operations Do Not Create Versions

Operations that fail **validation or application** do not increment the version number.

```
Invalid operation
      │
      ▼
Version remains unchanged
```

**Validation failures include:**
- Position out of range for the current content
- Negative or zero delete length
- Malformed operation structure

**Implementation:**
```go
func (r *Room) ApplyOperation(op Operation) (string, int) {
    // ...
    originalContent := r.Content
    
    apply(op)
    
    // Only increment version if content actually changed
    if r.Content != originalContent {
        r.Version++
    }
    // If content unchanged, version stays the same
}
```

**Example:**
- Current: Version 5, Content: "Hello"
- Operation: `INSERT("world", position=100)` — position out of range
- Result: Version remains 5, Content remains "Hello"

---

## Rule 4: Duplicate Operations Do Not Create Versions

When the **same operation ID** arrives more than once, the server silently ignores subsequent deliveries. The version does not change.

```
op-123 arrives
       │
       ▼
Version 5 → Version 6

Duplicate:
op-123 arrives again
       │
       ▼
Ignore

Version remains: 6
```

This provides **idempotency** for network retries and message redelivery.

**Implementation:**
```go
func (r *Room) ApplyOperation(op Operation) (string, int) {
    // Idempotency check
    if op.ID != "" && r.ProcessedOperations[op.ID] {
        return r.Content, r.Version  // <-- return unchanged
    }
    
    // ... apply operation ...
    
    // Mark as processed
    if op.ID != "" {
        r.ProcessedOperations[op.ID] = true
    }
}
```

**Example:**
- Operations processed: `["op-a", "op-b", "op-c"]`
- Op "op-b" is redelivered (e.g., after a network timeout)
- Server detects duplicate, returns current state without incrementing version

---

## Rule 5: Versions Are Strictly Increasing

The version sequence is always strictly increasing. A version can never decrease or stay the same after a successful operation.

```
1 → 2 → 3 → 4 → 5

Never:
1 → 3 → 2
```

**Monotonicity guarantees:**
- Every successful operation produces `version + 1`
- No operation can produce a version ≤ current version
- The sequence `1, 2, 3, ...` is immutable once created

**Why this matters:**
- Clients can use version numbers to detect if their state is behind the server
- History can be reconstructed deterministically from version order
- No version reuse or gaps are allowed

---

## Version Check Protocol

When a client submits an operation, it includes its `BaseVersion` — the version it last saw. The server validates this against its current version:

| Client BaseVersion | Server Version | Result |
|-------------------|----------------|--------|
| `0`               | Any            | ✅ Apply (backward compatibility) |
| `N`               | `N`            | ✅ Apply directly |
| `N`               | `> N`          | ❌ Reject with `STALE_OPERATION` |
| `N`               | `< N`          | ❌ Reject with `VERSION_MISMATCH` |

**Backward compatibility:** `BaseVersion = 0` means "no version tracking" and is accepted regardless of server version.

**Stale detection:** If `BaseVersion < ServerVersion`, the client is behind and must first fetch missing operations before submitting new ones.

---

## Operation Lifecycle

```
1. Client has document at Version N
2. Client creates operation with BaseVersion = N
3. Client sends operation to server
4. Server validates operation
5. Server checks BaseVersion == Server.Version
6. Server applies operation
      ├── Duplicate? → ignore, return current version
      ├── Invalid?   → reject, version unchanged
      └── Valid      → apply, Version = N + 1
7. Server broadcasts updated document with Version = N + 1
8. Client updates its version to N + 1
```

---

## Conflict Resolution: Stale Operation Transformation

When a client submits a stale operation (based on an outdated version), the server **transforms** it against the missing operations rather than rejecting it. This allows the client's intent to be preserved.

### Stale Client (Transform Instead of Reject)

```
Initial State
  Room Version: 10
  Content: "Hello World"
  Length: 11
  Position 5: space between "Hello" and "World"

Client A sends (BaseVersion: 10):
  INSERT(" beautiful", position=5)

Client B sends (BaseVersion: 10):
  DELETE(position=5, length=6)

Sequence:
  1. Client A's operation arrives first
  2. Server applies INSERT → Version 11, Content: "Hello beautiful World"
  3. Client B's operation arrives (BaseVersion=10, but server is at 11)
  4. Server detects stale operation
  5. Server transforms B's DELETE against A's INSERT
  6. Transformed DELETE at position 15, length 6 → deletes " World"
  7. Apply transformed DELETE → Version 12, Content: "Hello beautiful"
  8. Server broadcasts transformed operation to all clients
```

**The key insight:** The server no longer simply says "Your version is old. Try again." It intelligently reconciles the operation by transforming it against the missing operations.

### Step-by-Step Transformation

**Step 1: Initial State**
```
Document: "Hello World"
Version: 10
```

**Step 2: Apply Client A's Operation**

Client A: `INSERT(" beautiful", position=5)`

```go
// Server applies directly
// " beautiful" is 10 characters (including the leading space)
content = content[:5] + " beautiful" + content[5:]
// = "Hello" + " beautiful" + " World"
// = "Hello beautiful World"
// Length: 11 + 10 = 21
```

Result:
- Version: 11
- Content: "Hello beautiful World"
- Length: 21

**Step 3: Client B's Operation Arrives (Stale)**

Client B's operation was based on Version 10:
- Operation: `DELETE(position=5, length=6)`
- At Version 10, this would delete " World" (6 chars starting at index 5)

However, server is at Version 11. The position 5 no longer refers to the same text!

**Step 4: Transform Client B's Operation**

Detect 1 missing operation (Version 11):
```go
missingOps := r.GetOperationsAfter(10)
// Returns: [{11: INSERT(" beautiful", position=5)}]
```

Transform B's delete against A's insert:
```go
transformed = Transform(
    incoming:  DELETE(position=5, length=6),
    applied:   INSERT(position=5, text=" beautiful")
)

// Using transformDeleteAfterInsert(5, 6, INSERT{5, " beautiful"})
// Since incomingPos (5) is NOT < applied.Position (5)
// return (5 + len(" beautiful"), 6) = (5 + 10, 6) = (15, 6)
```

**Step 5: Verify Transformation is Correct**

What was B's intent? Delete " World" (6 characters from position 5)

After A's INSERT at position 5, where is the text B wanted to delete?
- Original text at positions 5-10: " World" (6 chars)
- A inserted " beautiful" (10 chars) at position 5
- " World" is now shifted to position 5+10 = 15

Transformed DEL at position 15, length 6:
```go
"Hello beautiful World"[0:15]  = "Hello beautiful"
"Hello beautiful World"[15:21] = " World"  ← deleted
"Hello beautiful World"[21:]   = ""
Result: "Hello beautiful"
```

This matches what applying B's delete to the original text would produce. ✓

**Step 6: Apply Transformed Operation**

```go
r.ApplyOperationWithVersion(transformedOp)
// Content = content[:15] + content[21:]
// = "Hello beautiful" + ""
// = "Hello beautiful"
```

Result:
- Version: 12
- Content: "Hello beautiful"

**Step 7: Broadcast to All Clients**

The transformed operation is broadcast to all clients:
```json
{
  "type": "operation",
  "id": "op-b",
  "base_version": 10,
  "version": 12,
  "position": 15,
  "length": 6,
  "text": ""
}
```

Clients update their state to Version 12 with content "Hello beautiful".

### Implementation in Handler

```go
if result.Err == ErrStaleOperation {
    // Fetch missing operations
    clientBaseVersion := op.BaseVersion
    missingOps := r.GetOperationsAfter(int(clientBaseVersion))
    
    // Transform the stale operation against each missing operation
    transformedOp := op
    for _, entry := range missingOps {
        transformedOp = Transform(transformedOp, entry.Operation)
    }
    
    // Apply the transformed operation with guaranteed new version
    _, newVersion := r.ApplyOperationWithVersion(transformedOp)
    
    // Broadcast the transformed operation
    BroadcastTransformedOperation(transformedOp, newVersion)
}
```

### Transformation Rules for This Example

| Operation Pair | Outcome |
|---------------|---------|
| DELETE arriving after INSERT | Shift position right by `len(applied.Text)` if delete is at or after insert position |
| DELETE(position=5, len=6) after INSERT(position=5, "beautiful") | position becomes 5+10=15, len stays 6 |

### Deterministic Conflict Policy

The conflict is resolved deterministically:
1. **Ordered operations**: The server applies operations in arrival order
2. **Transformation**: Later operations are mathematically adjusted
3. **Intent preservation**: The final document reflects both clients' intentions

This ensures all clients converge to the same document state.

---

## Failure Modes
### Stale Client (Back in Time)
```
Server: Version 5, Content "Hello World"
Client: Version 3 (stale)
Client sends: BaseVersion = 3

Result: Operation is TRANSFORMED against missing operations (v4, v5)
Server broadcasts the transformed operation
Server version becomes: 6
```

### Future Client (Clock Skew / Bug)
```
Server: Version 5
Client: Version 99 (supposed future)
Client sends: BaseVersion = 99

Result: VERSION_MISMATCH error
Server version remains: 5
```

### Operation Validation Failure
```
Server: Version 5, Content "Hi"
Client: Version 5
Operation: INSERT("X", position=100)

Result: Validation error (position out of range)
Server version remains: 5
```

---

## Reference: Code Locations

| Component | File | Purpose |
|-----------|------|---------|
| Version field | `internal/websocket/room.go` | Stores document version |
| ApplyOperation | `internal/websocket/room.go` | Atomic version increment |
| Duplicate check | `internal/websocket/processor.go` | Idempotency enforcement |
| Version validation | `internal/websocket/processor.go` | BaseVersion matching |
| History tracking | `internal/websocket/room.go` | Immutable operation log |
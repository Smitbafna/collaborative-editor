# SyncPad

A real-time collaborative text editor built with Go and WebSockets. Multiple users can edit the same document simultaneously with operational transformation for conflict resolution.

## Features

- **Real-time Collaboration**: Multiple clients can edit the same document simultaneously via WebSocket connections
- **Operational Transformation**: Automatically resolves conflicting insert and delete operations
- **Document Rooms**: Isolated document spaces identified by unique IDs (e.g., `/ws/documents/document-123`)
- **Version Tracking**: Each document has a monotonically increasing version number for every operation applied
- **Idempotent Operations**: Duplicate operations are detected and ignored to prevent data corruption
- **Document Snapshots**: New clients receive the full current state upon joining a room
- **Missing Operation Sync**: Clients detect version gaps and automatically request missing operations
- **Auto Cleanup**: Empty rooms are removed when all clients disconnect

## Architecture

```
┌─────────────┐     ┌─────────────────────────────┐
│   Client A  │────▶│                             │
│ (Browser)   │     │         HTTP Server         │
├─────────────┤     │   - Serves static files     │
│   Client B  │────▶│   - WebSocket upgrade       │
│ (Browser)   │     │   - /health endpoint        │
└─────────────┘     └───────────┬─────────────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │       Hub             │
                    │ - Manages WebSocket   │
                    │   connections         │
                    │ - Routes to rooms     │
                    │ - Broadcasts messages │
                    └───────────┬───────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │    RoomManager        │
                    │ - Creates rooms       │
                    │ - Joins/leaves clients│
                    │ - Cleans up empty     │
                    │   rooms               │
                    └───────────┬───────────┘
                                │
              ┌─────────────────┼─────────────────┐
              ▼                 ▼                 ▼
      ┌───────────────┐ ┌───────────────┐ ┌───────────────┐
      │   Room A      │ │   Room B      │ │   Room C      │
      │ - Doc content │ │ - Doc content │ │ - Doc content │
      │ - Version     │ │ - Version     │ │ - Version     │
      │ - History     │ │ - History     │ │ - History     │
      │ - Lock        │ │ - Lock        │ │ - Lock        │
      └───────────────┘ └───────────────┘ └───────────────┘
```

### Component Responsibilities

- **Server (`cmd/server/main.go`)**: Starts HTTP server, serves frontend, upgrades to WebSocket, graceful shutdown
- **Hub (`internal/websocket/handler.go`)**: Accepts WebSocket connections, routes clients to document rooms, handles operation processing and broadcasting
- **Room (`internal/websocket/room.go`)**: Holds document state, version history, and client list for a single document
- **Operation Processor (`internal/websocket/processor.go`)**: Validates, applies, and idempotently tracks operations
- **Transformer (`internal/websocket/transform.go`)**: Implements operational transformation for all conflict cases
- **Client (`internal/websocket/client.go`)**: Wraps WebSocket connection with send buffer and base version tracking

## Message Format

### Client → Server: Operation

```json
{
  "id": "op-123",
  "type": "insert",
  "position": 5,
  "text": "Hello",
  "base_version": 5
}
```

For delete operations:
```json
{
  "id": "op-124",
  "type": "delete",
  "position": 5,
  "length": 6,
  "base_version": 5
}
```

### Client → Server: Request Missing Operations

```json
{
  "type": "request_missing_operations",
  "base_version": 5
}
```

### Server → Client: Document Snapshot (sent on join)

```json
{
  "type": "document_snapshot",
  "content": "current document text",
  "version": 10
}
```

### Server → Client: Operation Broadcast

```json
{
  "type": "operation",
  "operation": {
    "id": "op-125",
    "type": "insert",
    "position": 0,
    "text": "Hi",
    "base_version": 10
  },
  "version": 11
}
```

### Server → Client: Error

```json
{
  "type": "error",
  "message": "stale operation"
}
```

## Project Structure

```
syncpad/
├── cmd/
│   ├── server/
│   │   └── main.go          # HTTP server with WebSocket support
│   └── client/
│       └── client.go        # Demo client for testing
├── internal/
│   └── websocket/
│       ├── client.go        # Client connection wrapper
│       ├── handler.go       # WebSocket connection handler
│       ├── room.go          # Room management and document state
│       ├── operation.go     # Operation validation and types
│       ├── processor.go     # Operation processing pipeline
│       ├── transform.go     # Operational transformation logic
│       ├── message.go       # Message envelope types
│       ├── stale_error.go   # Stale operation error
│       ├── *_test.go        # Unit tests
│       └── VERSIONING.md    # Version system documentation
├── web/
│   └── index.html           # Frontend demo
├── go.mod
├── go.sum
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.25.0 or later

### Installation

```bash
# Clone the repository
git clone git@github.com:Smitbafna/collaborative-editor.git
cd syncpad

# Install dependencies
go mod download

# Run tests (optional)
go test ./...
```

### Running the Server

```bash
# Start the server (defaults to port 8080)
PORT=8080 go run ./cmd/server

# The server is now running at http://localhost:8080/
# Health check: http://localhost:8080/health
# WebSocket endpoint: ws://localhost:8080/ws/documents/{document-id}
```

### Running the Demo Client

```bash
# In a separate terminal
go run ./cmd/client

# The demo client will:
# 1. Connect to the server
# 2. Insert "Hello World" at position 0
# 3. Delete 5 characters at position 0
# 4. Display server responses
```

### Testing in Browser

Open `web/index.html` in multiple browser tabs to see real-time collaboration between tabs.

## How It Works

### Operation Lifecycle

1. **Client sends operation** with a `base_version` representing the document version it was based on
2. **Server validates** the operation under the room write lock:
   - Checks for duplicate operation IDs
   - Verifies the `base_version` matches the current server version
   - Validates position/length bounds against current content
3. **If stale**: Operation version conflicts with current version. Server transforms the operation against missing operations and applies it as-isolated. If transformation produces an invalid operation, it's converted to a no-op but still assigned a new version.
4. **If valid**: Operation is applied, version is incremented, and broadcast to all clients in the room.
5. **Clients update** their local document and version number from the broadcast.

### Operational Transformation

When two clients edit the same document concurrently, operations may conflict. The server uses operational transformation to resolve conflicts:

```
Client A (v5)          Client B (v5)
       │                      │
 insert "X" at 3    insert "Y" at 3
       │                      │
       ▼                      ▼
Server receives first operation successfully
       │
       ▼
 Server transforms second operation
 insert "Y" at 3  →  insert "Y" at 4
 (because "X" now occupies position 3)
       │
       ▼
 Second operation applied successfully
```

Supported transformations:
- Insert after insert: Adjust positions for inserted text
- Insert after delete: Adjust positions for removed range
- Delete after insert: Adjust positions for inserted text
- Delete after delete: Compute remaining range, or no-op if delete was already removed

### Versioning

- Versions start at 0
- Every successful operation increments the version by 1
- Clients include their version in every operation (`base_version`)
- Version gaps trigger missing operation sync requests
- Sentry: Because transforming a stale operation can produce invalid results (positions out of bounds, etc.), the processor applies the following safeguard: after transformation against all missing operations, it validates the result against the current document content. If validation fails, the operation is downgraded to a no-op before being applied, preventing crashes or data corruption.

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test package
go test -v ./internal/websocket/
```

### Key Files to Study

| File | Purpose |
|------|---------|
| `transform.go` | Core operational transformation logic |
| `handler.go` | WebSocket message routing and stale operation handling |
| `room.go` | Document state, version tracking, operation history |
| `processor.go` | Operation pipeline: validation, atomic apply, versioning |

## Message Protocols

### WebSocket Endpoint

`/ws/documents/{document-id}`

- Protocol: `ws://` or `wss://`
- Path: `/ws/documents/{document-id}` where `{document-id}` is any string identifying the document
- Close: Server may close connections for slow clients (write buffer full)

### HTTP Endpoints

| Path | Method | Description |
|------|--------|-------------|
| `/health` | GET | Returns JSON `{"status": "ok"}` |
| `/ws/documents/{id}` | GET (upgrade) | WebSocket endpoint for document collaboration |
| `/` | GET | Serves static frontend files from `/web/` |

## Known Limitations

- No persistence: Document state is lost when server restarts
- No authentication: Any client can join any document ID
- No undo/redo: Operations are append-only
- No document creation: Documents are auto-created when first client joins
- No message size limit: Connections are closed on write buffer saturation

## Contributing

1. Fork the repository
2. Create a feature branch
3. Run tests to verify your changes
4. Submit a pull request

## License

MIT
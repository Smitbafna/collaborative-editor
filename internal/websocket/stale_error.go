package websocket

import "fmt"

// StaleOperationError is returned when a client submits an operation
// based on an outdated document version.
type StaleOperationError struct {
	BaseVersion   int64
	CurrentVersion int64
}

func (e *StaleOperationError) Error() string {
	return fmt.Sprintf("operation is based on an outdated document version")
}

func (e *StaleOperationError) Code() string {
	return "STALE_OPERATION"
}

func (e *StaleOperationError) BaseVersionValue() int64 {
	return e.BaseVersion
}

func (e *StaleOperationError) CurrentVersionValue() int64 {
	return e.CurrentVersion
}
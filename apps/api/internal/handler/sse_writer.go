package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/stream"
)

// SSEWriter is re-exported from the stream package for backward compatibility.
type SSEWriter = stream.SSEWriter

// NewSSEWriter delegates to stream.NewSSEWriter.
func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	return stream.NewSSEWriter(w)
}

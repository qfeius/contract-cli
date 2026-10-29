package tracecontext

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

const (
	HeaderTraceparent = "traceparent"
	HeaderLogID       = "X-Log-Id"

	traceIDBytes = 16
	spanIDBytes  = 8
)

// RequestTrace identifies a command or standalone logical request. Apply creates
// a fresh parent span ID for each explicit HTTP sending attempt.
type RequestTrace struct {
	TraceID string
}

func NewRequestTrace() (RequestTrace, error) {
	traceID, err := randomNonZeroHex(traceIDBytes)
	if err != nil {
		return RequestTrace{}, fmt.Errorf("generate trace id: %w", err)
	}
	return RequestTrace{TraceID: traceID}, nil
}

// Apply replaces any caller-supplied correlation headers so traceparent and
// X-Log-Id cannot disagree on the trace ID sent to the server.
func (trace RequestTrace) Apply(headers http.Header) error {
	if headers == nil {
		return fmt.Errorf("trace headers are required")
	}
	decodedTraceID, err := hex.DecodeString(trace.TraceID)
	if err != nil || len(decodedTraceID) != traceIDBytes || hex.EncodeToString(decodedTraceID) != trace.TraceID || allZero(decodedTraceID) {
		return fmt.Errorf("trace id must contain %d lowercase hexadecimal characters", traceIDBytes*2)
	}

	spanID, err := randomNonZeroHex(spanIDBytes)
	if err != nil {
		return fmt.Errorf("generate span id: %w", err)
	}
	headers.Set(HeaderTraceparent, "00-"+trace.TraceID+"-"+spanID+"-01")
	headers.Set(HeaderLogID, trace.TraceID)
	return nil
}

func randomNonZeroHex(size int) (string, error) {
	buffer := make([]byte, size)
	for {
		if _, err := rand.Read(buffer); err != nil {
			return "", err
		}
		if !allZero(buffer) {
			return hex.EncodeToString(buffer), nil
		}
	}
}

func allZero(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}

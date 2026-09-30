package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ReconcileRequest carries exact retained request evidence, never a new mutation.
// The service only reads accepted authority under its normal serialization lock.
type ReconcileRequest struct {
	ServiceID string            `json:"service_id"`
	ClientID  string            `json:"client_id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Body      json.RawMessage   `json:"body"`
	Headers   map[string]string `json:"headers"`
}

func ReconcileDigest(q ReconcileRequest) string {
	b, _ := json.Marshal(q)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type TransactionStatus struct {
	RequestDigest string          `json:"request_digest"`
	ServiceID     string          `json:"service_id"`
	ClientID      string          `json:"client_id"`
	RequestID     string          `json:"request_id,omitempty"`
	Status        string          `json:"status"`
	Proof         string          `json:"proof"`
	Outcome       string          `json:"outcome,omitempty"`
	RequestHash   string          `json:"request_hash,omitempty"`
	Results       []ChangedObject `json:"results,omitempty"`
	// Response is reconstructed only from established history/current-state proof;
	// it is not a stored original HTTP response or an idempotency receipt.
	Response json.RawMessage `json:"response,omitempty"`
}

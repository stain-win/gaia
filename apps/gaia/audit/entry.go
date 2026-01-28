// Package audit provides structured audit logging for the Gaia daemon.
// It follows HashiCorp Vault's audit logging best practices with support for
// multiple backends (Internal/BoltDB, File/Stdout, Webhook/gRPC).
package audit

import (
	"time"
)

// EntryType indicates whether the audit entry is for a request or response.
type EntryType string

const (
	// EntryTypeRequest indicates an audit entry for an incoming request.
	EntryTypeRequest EntryType = "request"
	// EntryTypeResponse indicates an audit entry for an outgoing response.
	EntryTypeResponse EntryType = "response"
)

// Auth contains authentication information about the client making the request.
type Auth struct {
	// ClientIdentity is the Common Name (CN) from the client's mTLS certificate.
	ClientIdentity string `json:"client_identity"`
	// ClientType indicates whether this is an admin or regular client.
	ClientType string `json:"client_type,omitempty"`
}

// Request contains details about the incoming gRPC request.
type Request struct {
	// ID is a unique identifier for this request, used to correlate request/response entries.
	ID string `json:"id"`
	// Operation is the gRPC method name (e.g., "/gaia.GaiaAdmin/AddSecret").
	Operation string `json:"operation"`
	// Path is the resource path being accessed (e.g., "client-a/production/db_password").
	Path string `json:"path,omitempty"`
	// Namespace is the secret namespace being accessed.
	Namespace string `json:"namespace,omitempty"`
	// ClientName is the target client name for admin operations.
	ClientName string `json:"client_name,omitempty"`
	// RemoteAddr is the client's network address.
	RemoteAddr string `json:"remote_addr"`
	// Data contains non-sensitive request parameters.
	Data map[string]any `json:"data,omitempty"`
}

// Response contains details about the outgoing gRPC response.
type Response struct {
	// Success indicates whether the operation succeeded.
	Success bool `json:"success"`
	// StatusCode is the gRPC status code.
	StatusCode int `json:"status_code"`
	// Data contains non-sensitive response data.
	Data map[string]any `json:"data,omitempty"`
}

// Error contains details about any error that occurred.
type Error struct {
	// Message is the error message (sensitive data should be redacted).
	Message string `json:"message"`
	// Code is the error code if applicable.
	Code string `json:"code,omitempty"`
}

// Entry is a single audit log entry following the Who, What, When, Where, Why pattern.
type Entry struct {
	// Type indicates whether this is a request or response entry.
	Type EntryType `json:"type"`

	// Time is the timestamp of the audit event in RFC3339Nano format.
	Time time.Time `json:"time"`

	// Auth contains the "Who" - client identity information.
	Auth Auth `json:"auth"`

	// Request contains the "What" and "Where" - what operation on what resource.
	Request Request `json:"request"`

	// Response contains the outcome (only for response entries).
	Response *Response `json:"response,omitempty"`

	// Error contains error details if the operation failed.
	Error *Error `json:"error,omitempty"`
}

// NewRequestEntry creates a new audit entry for an incoming request.
func NewRequestEntry(requestID, operation, remoteAddr string, auth Auth) *Entry {
	return &Entry{
		Type: EntryTypeRequest,
		Time: time.Now().UTC(),
		Auth: auth,
		Request: Request{
			ID:         requestID,
			Operation:  operation,
			RemoteAddr: remoteAddr,
		},
	}
}

// NewResponseEntry creates a new audit entry for an outgoing response.
func NewResponseEntry(requestID, operation, remoteAddr string, auth Auth, success bool, statusCode int) *Entry {
	return &Entry{
		Type: EntryTypeResponse,
		Time: time.Now().UTC(),
		Auth: auth,
		Request: Request{
			ID:         requestID,
			Operation:  operation,
			RemoteAddr: remoteAddr,
		},
		Response: &Response{
			Success:    success,
			StatusCode: statusCode,
		},
	}
}

// WithPath sets the resource path on the entry.
func (e *Entry) WithPath(path string) *Entry {
	e.Request.Path = path
	return e
}

// WithNamespace sets the namespace on the entry.
func (e *Entry) WithNamespace(namespace string) *Entry {
	e.Request.Namespace = namespace
	return e
}

// WithClientName sets the target client name on the entry.
func (e *Entry) WithClientName(clientName string) *Entry {
	e.Request.ClientName = clientName
	return e
}

// WithData sets non-sensitive request data on the entry.
func (e *Entry) WithData(data map[string]any) *Entry {
	e.Request.Data = data
	return e
}

// WithResponseData sets non-sensitive response data on the entry.
func (e *Entry) WithResponseData(data map[string]any) *Entry {
	if e.Response != nil {
		e.Response.Data = data
	}
	return e
}

// WithError sets error details on the entry.
func (e *Entry) WithError(message, code string) *Entry {
	e.Error = &Error{
		Message: message,
		Code:    code,
	}
	return e
}

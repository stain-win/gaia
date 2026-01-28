package audit

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Interceptor provides gRPC interceptors for audit logging.
type Interceptor struct {
	logger *Logger
}

// NewInterceptor creates a new audit interceptor.
func NewInterceptor(logger *Logger) *Interceptor {
	return &Interceptor{logger: logger}
}

// UnaryServerInterceptor returns a gRPC unary interceptor for audit logging.
func (i *Interceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !i.logger.IsEnabled() {
			return handler(ctx, req)
		}

		// Generate unique request ID
		requestID := uuid.New().String()

		// Extract client identity and remote address
		auth, remoteAddr := extractAuthInfo(ctx)

		// Create request entry
		reqEntry := NewRequestEntry(requestID, info.FullMethod, remoteAddr, auth)

		// Extract non-sensitive request data
		reqData := extractRequestData(info.FullMethod, req)
		if len(reqData) > 0 {
			reqEntry.WithData(reqData)
		}

		// Log the request
		i.logger.LogRequest(reqEntry)

		// Invoke the handler
		resp, err := handler(ctx, req)

		// Create response entry
		statusCode := codes.OK
		if err != nil {
			statusCode = status.Code(err)
		}

		respEntry := NewResponseEntry(
			requestID,
			info.FullMethod,
			remoteAddr,
			auth,
			err == nil,
			int(statusCode),
		)

		// Copy request metadata to response entry
		respEntry.Request.Path = reqEntry.Request.Path
		respEntry.Request.Namespace = reqEntry.Request.Namespace
		respEntry.Request.ClientName = reqEntry.Request.ClientName

		// Add error details if present
		if err != nil {
			respEntry.WithError(err.Error(), statusCode.String())
		}

		// Extract response data
		if resp != nil {
			respData := extractResponseData(info.FullMethod, resp)
			if len(respData) > 0 {
				respEntry.WithResponseData(respData)
			}
		}

		// Log the response
		i.logger.LogResponse(respEntry)

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream interceptor for audit logging.
func (i *Interceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if !i.logger.IsEnabled() {
			return handler(srv, ss)
		}

		// Generate unique request ID
		requestID := uuid.New().String()

		// Extract client identity and remote address
		auth, remoteAddr := extractAuthInfo(ss.Context())

		// Create request entry for stream start
		reqEntry := NewRequestEntry(requestID, info.FullMethod, remoteAddr, auth)
		reqEntry.WithData(map[string]any{
			"stream_type":    "server_streaming",
			"client_streams": info.IsClientStream,
			"server_streams": info.IsServerStream,
		})

		i.logger.LogRequest(reqEntry)

		// Invoke the handler
		err := handler(srv, ss)

		// Create response entry for stream end
		statusCode := codes.OK
		if err != nil {
			statusCode = status.Code(err)
		}

		respEntry := NewResponseEntry(
			requestID,
			info.FullMethod,
			remoteAddr,
			auth,
			err == nil,
			int(statusCode),
		)

		if err != nil {
			respEntry.WithError(err.Error(), statusCode.String())
		}

		i.logger.LogResponse(respEntry)

		return err
	}
}

// extractAuthInfo extracts client identity from the gRPC context.
func extractAuthInfo(ctx context.Context) (Auth, string) {
	auth := Auth{
		ClientIdentity: "unknown",
		ClientType:     "unknown",
	}
	remoteAddr := "unknown"

	p, ok := peer.FromContext(ctx)
	if ok {
		remoteAddr = p.Addr.String()

		// Extract client identity from mTLS certificate
		if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if len(tlsInfo.State.PeerCertificates) > 0 {
				cert := tlsInfo.State.PeerCertificates[0]
				auth.ClientIdentity = cert.Subject.CommonName

				// Determine client type based on method or certificate OU
				if len(cert.Subject.OrganizationalUnit) > 0 {
					auth.ClientType = cert.Subject.OrganizationalUnit[0]
				}
			}
		}
	}

	return auth, remoteAddr
}

// extractRequestData extracts non-sensitive data from request messages.
// This uses reflection-free approach to avoid importing proto message types.
func extractRequestData(method string, req any) map[string]any {
	data := make(map[string]any)

	// Skip if nil
	if req == nil {
		return data
	}

	// Extract common fields based on method patterns
	// We use type assertions for known patterns without importing proto types

	switch {
	case strings.HasSuffix(method, "/AddSecret"):
		if m := getField(req, "Namespace"); m != "" {
			data["namespace"] = m
		}
		if m := getField(req, "Id"); m != "" {
			data["secret_id"] = m
		}
		if m := getField(req, "ClientName"); m != "" {
			data["client_name"] = m
		}
		// Note: "Value" is intentionally NOT extracted (sensitive)

	case strings.HasSuffix(method, "/GetSecret"):
		if m := getField(req, "Namespace"); m != "" {
			data["namespace"] = m
		}
		if m := getField(req, "Id"); m != "" {
			data["secret_id"] = m
		}

	case strings.HasSuffix(method, "/DeleteSecret"):
		if m := getField(req, "Namespace"); m != "" {
			data["namespace"] = m
		}
		if m := getField(req, "Id"); m != "" {
			data["secret_id"] = m
		}
		if m := getField(req, "ClientName"); m != "" {
			data["client_name"] = m
		}

	case strings.HasSuffix(method, "/RegisterClient"):
		if m := getField(req, "ClientName"); m != "" {
			data["client_name"] = m
		}

	case strings.HasSuffix(method, "/RevokeClient"):
		if m := getField(req, "ClientName"); m != "" {
			data["client_name"] = m
		}

	case strings.HasSuffix(method, "/ListSecrets"):
		if m := getField(req, "ClientName"); m != "" {
			data["client_name"] = m
		}

	case strings.HasSuffix(method, "/ListNamespaces"):
		if m := getField(req, "ClientName"); m != "" {
			data["client_name"] = m
		}

	case strings.HasSuffix(method, "/GetPolicy"), strings.HasSuffix(method, "/DeletePolicy"):
		if m := getField(req, "ClientName"); m != "" {
			data["client_name"] = m
		}

	case strings.HasSuffix(method, "/SetPolicy"):
		if m := getField(req, "ClientName"); m != "" {
			data["client_name"] = m
		}

	case strings.HasSuffix(method, "/Unlock"):
		// Note: Passphrase is intentionally NOT extracted (sensitive)
		data["operation"] = "unlock"
	}

	return data
}

// extractResponseData extracts non-sensitive data from response messages.
func extractResponseData(method string, resp any) map[string]any {
	data := make(map[string]any)

	if resp == nil {
		return data
	}

	// Extract success indicator if present
	if success := getBoolField(resp, "Success"); success {
		data["success"] = true
	}

	// Method-specific response data
	switch {
	case strings.HasSuffix(method, "/ListClients"):
		// Don't include actual client list, just count
		if clients := getSliceLen(resp, "Clients"); clients >= 0 {
			data["client_count"] = clients
		}

	case strings.HasSuffix(method, "/ListSecrets"):
		// Don't include secrets, just namespace count
		if namespaces := getSliceLen(resp, "Namespaces"); namespaces >= 0 {
			data["namespace_count"] = namespaces
		}

	case strings.HasSuffix(method, "/ListPolicies"):
		if policies := getSliceLen(resp, "Policies"); policies >= 0 {
			data["policy_count"] = policies
		}

	case strings.HasSuffix(method, "/GetStatus"):
		if statusVal := getField(resp, "Status"); statusVal != "" {
			data["daemon_status"] = statusVal
		}
	}

	return data
}

// getField uses reflection to safely get a string field from a struct.
func getField(obj any, fieldName string) string {
	// Use type assertion patterns for common proto message fields
	// This avoids heavy reflection usage

	type hasNamespace interface{ GetNamespace() string }
	type hasId interface{ GetId() string }
	type hasClientName interface{ GetClientName() string }
	type hasStatus interface{ GetStatus() string }

	switch fieldName {
	case "Namespace":
		if v, ok := obj.(hasNamespace); ok {
			return v.GetNamespace()
		}
	case "Id":
		if v, ok := obj.(hasId); ok {
			return v.GetId()
		}
	case "ClientName":
		if v, ok := obj.(hasClientName); ok {
			return v.GetClientName()
		}
	case "Status":
		if v, ok := obj.(hasStatus); ok {
			return v.GetStatus()
		}
	}

	return ""
}

// getBoolField safely gets a bool field.
func getBoolField(obj any, fieldName string) bool {
	type hasSuccess interface{ GetSuccess() bool }

	if fieldName == "Success" {
		if v, ok := obj.(hasSuccess); ok {
			return v.GetSuccess()
		}
	}
	return false
}

// getSliceLen safely gets the length of a slice field.
// Note: This is a placeholder for future implementation using reflection.
func getSliceLen(_ any, _ string) int {
	// For simplicity, we return -1 if we can't determine the length
	// A more complete implementation would use reflection
	return -1
}

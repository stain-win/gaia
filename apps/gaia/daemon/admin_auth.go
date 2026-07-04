package daemon

import (
	"context"
	"crypto/x509"
	"strings"

	"github.com/stain-win/gaia/apps/gaia/certs"
	gaialog "github.com/stain-win/gaia/apps/gaia/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// adminServicePrefix is the fully-qualified gRPC method prefix for the privileged
// GaiaAdmin service. Only calls to methods under this prefix are subject to the
// admin-authorization check; the read-only GaiaClient service is unaffected.
const adminServicePrefix = "/gaia.GaiaAdmin/"

// isAdminIdentity reports whether the given peer certificate is authorized to call
// the GaiaAdmin service. A certificate qualifies if it carries the admin
// Organizational Unit marker (certs.AdminOU) or if its Common Name appears in the
// configured allow-list. The OU marker is the authoritative signal: it is only
// stamped by local administrative tooling that has direct access to the CA key and
// is never applied to certificates issued through the RegisterClient RPC, so an
// application client cannot obtain one. The CN allow-list exists for backward
// compatibility with admin certificates issued before the OU marker was introduced.
func isAdminIdentity(cert *x509.Certificate, adminCommonNames []string) bool {
	if cert == nil {
		return false
	}
	for _, ou := range cert.Subject.OrganizationalUnit {
		if ou == certs.AdminOU {
			return true
		}
	}
	for _, cn := range adminCommonNames {
		if cn != "" && cert.Subject.CommonName == cn {
			return true
		}
	}
	return false
}

// authorizeAdmin enforces admin authorization for calls to the GaiaAdmin service.
// It returns nil for non-admin methods and for authorized admin callers, and a
// gRPC status error otherwise. The transport already guarantees a CA-verified
// client certificate (RequireAndVerifyClientCert); this adds the missing identity
// check so that possessing any CA-issued client certificate is not, by itself,
// sufficient to invoke privileged operations.
func (d *Daemon) authorizeAdmin(ctx context.Context, fullMethod string) error {
	if !strings.HasPrefix(fullMethod, adminServicePrefix) {
		return nil
	}

	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "no peer information available")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "a client certificate is required for admin operations")
	}

	cert := tlsInfo.State.PeerCertificates[0]

	var adminCommonNames []string
	if d.config != nil {
		adminCommonNames = d.config.TLS.AdminCommonNames
	}

	if !isAdminIdentity(cert, adminCommonNames) {
		gaialog.Get().Warn("admin RPC rejected: caller certificate is not authorized",
			"method", fullMethod,
			"client_cn", cert.Subject.CommonName,
			"remote_addr", p.Addr.String())
		return status.Error(codes.PermissionDenied, "caller is not authorized for admin operations")
	}

	return nil
}

// authorizePeer runs all per-call authorization checks: admin gating for the
// GaiaAdmin service and revocation enforcement for the GaiaClient service.
func (d *Daemon) authorizePeer(ctx context.Context, fullMethod string) error {
	if err := d.authorizeAdmin(ctx, fullMethod); err != nil {
		return err
	}
	return d.checkPeerRevocation(ctx, fullMethod)
}

// authUnaryInterceptor gates unary RPCs on peer authorization.
func (d *Daemon) authUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := d.authorizePeer(ctx, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// authStreamInterceptor gates streaming RPCs on peer authorization.
func (d *Daemon) authStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := d.authorizePeer(ss.Context(), info.FullMethod); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"testing"

	"github.com/stain-win/gaia/apps/gaia/certs"
	"github.com/stain-win/gaia/apps/gaia/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func TestIsAdminIdentity(t *testing.T) {
	tests := []struct {
		name      string
		cn        string
		ou        []string
		adminCNs  []string
		wantAdmin bool
	}{
		{
			name:      "admin OU marker grants access regardless of CN",
			cn:        "some-app",
			ou:        []string{certs.AdminOU},
			adminCNs:  nil,
			wantAdmin: true,
		},
		{
			name:      "CN in allow-list grants access",
			cn:        "gaia_client",
			ou:        nil,
			adminCNs:  []string{"gaia_client"},
			wantAdmin: true,
		},
		{
			name:      "ordinary client cert (no OU, CN not allow-listed) is rejected",
			cn:        "billing-service",
			ou:        nil,
			adminCNs:  []string{"gaia_client"},
			wantAdmin: false,
		},
		{
			name:      "empty allow-list requires OU marker",
			cn:        "gaia_client",
			ou:        nil,
			adminCNs:  nil,
			wantAdmin: false,
		},
		{
			name:      "empty CN entry never matches an empty client CN",
			cn:        "",
			ou:        nil,
			adminCNs:  []string{""},
			wantAdmin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := &x509.Certificate{
				Subject: pkix.Name{
					CommonName:         tt.cn,
					OrganizationalUnit: tt.ou,
				},
			}
			if got := isAdminIdentity(cert, tt.adminCNs); got != tt.wantAdmin {
				t.Fatalf("isAdminIdentity() = %v, want %v", got, tt.wantAdmin)
			}
		})
	}
}

func TestIsAdminIdentityNilCert(t *testing.T) {
	if isAdminIdentity(nil, []string{"gaia_client"}) {
		t.Fatal("nil certificate must never be treated as admin")
	}
}

// ctxWithCert builds a gRPC context carrying an mTLS peer whose leaf certificate
// has the given Common Name and Organizational Units.
func ctxWithCert(cn string, ou []string) context.Context {
	cert := &x509.Certificate{
		Subject: pkix.Name{CommonName: cn, OrganizationalUnit: ou},
	}
	return peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345},
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}},
		},
	})
}

func newTestDaemon() *Daemon {
	return &Daemon{config: config.NewDefaultConfig()}
}

func TestAuthorizeAdmin_AllowsClientServiceWithoutCert(t *testing.T) {
	d := newTestDaemon()
	// A GaiaClient method must not be gated by admin authorization.
	if err := d.authorizeAdmin(context.Background(), "/gaia.GaiaClient/GetSecret"); err != nil {
		t.Fatalf("client-service method should bypass admin auth, got: %v", err)
	}
}

func TestAuthorizeAdmin_RejectsNonAdminAdminCall(t *testing.T) {
	d := newTestDaemon()
	ctx := ctxWithCert("billing-service", nil)
	err := d.authorizeAdmin(ctx, "/gaia.GaiaAdmin/ListSecrets")
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for non-admin caller, got: %v", err)
	}
}

func TestAuthorizeAdmin_AllowsAdminByCN(t *testing.T) {
	d := newTestDaemon()
	ctx := ctxWithCert("gaia_client", nil)
	if err := d.authorizeAdmin(ctx, "/gaia.GaiaAdmin/RotatePassword"); err != nil {
		t.Fatalf("default admin CN should be authorized, got: %v", err)
	}
}

func TestAuthorizeAdmin_AllowsAdminByOU(t *testing.T) {
	d := newTestDaemon()
	d.config.TLS.AdminCommonNames = nil // strict mode: OU marker only
	ctx := ctxWithCert("some-admin", []string{certs.AdminOU})
	if err := d.authorizeAdmin(ctx, "/gaia.GaiaAdmin/Stop"); err != nil {
		t.Fatalf("admin OU marker should be authorized in strict mode, got: %v", err)
	}
}

func TestAuthorizeAdmin_RejectsAdminCallWithoutPeer(t *testing.T) {
	d := newTestDaemon()
	err := d.authorizeAdmin(context.Background(), "/gaia.GaiaAdmin/AddSecret")
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated when no peer certificate is present, got: %v", err)
	}
}

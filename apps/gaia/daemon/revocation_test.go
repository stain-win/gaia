package daemon

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCheckPeerRevocation(t *testing.T) {
	d := setupTestDaemon(t, "StrongTestPassphrase123!")
	defer d.LockDB()

	if err := d.RegisterClient("active-app"); err != nil {
		t.Fatalf("failed to register active client: %v", err)
	}
	if err := d.RegisterClient("revoked-app"); err != nil {
		t.Fatalf("failed to register revoked client: %v", err)
	}
	if err := d.RevokeClient("revoked-app"); err != nil {
		t.Fatalf("failed to revoke client: %v", err)
	}

	t.Run("revoked client is rejected before handlers", func(t *testing.T) {
		err := d.checkPeerRevocation(ctxWithCert("revoked-app", nil), "/gaia.GaiaClient/GetSecret")
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("expected PermissionDenied for revoked client, got: %v", err)
		}
	})

	t.Run("active client passes", func(t *testing.T) {
		if err := d.checkPeerRevocation(ctxWithCert("active-app", nil), "/gaia.GaiaClient/GetSecret"); err != nil {
			t.Fatalf("active client should pass revocation check, got: %v", err)
		}
	})

	t.Run("unregistered CN passes through to handlers", func(t *testing.T) {
		if err := d.checkPeerRevocation(ctxWithCert("gaia_client", nil), "/gaia.GaiaClient/ListSecrets"); err != nil {
			t.Fatalf("unregistered CN should pass through, got: %v", err)
		}
	})

	t.Run("admin service methods are not gated here", func(t *testing.T) {
		if err := d.checkPeerRevocation(context.Background(), "/gaia.GaiaAdmin/ListClients"); err != nil {
			t.Fatalf("admin methods should bypass revocation check, got: %v", err)
		}
	})

	t.Run("missing peer certificate is rejected", func(t *testing.T) {
		err := d.checkPeerRevocation(context.Background(), "/gaia.GaiaClient/GetSecret")
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected Unauthenticated without a peer certificate, got: %v", err)
		}
	})
}

func TestCheckPeerRevocation_LockedDaemonPassesThrough(t *testing.T) {
	d := setupTestDaemon(t, "StrongTestPassphrase123!")
	d.LockDB()

	// While locked, the check defers to handlers, which report FailedPrecondition.
	if err := d.checkPeerRevocation(ctxWithCert("any-app", nil), "/gaia.GaiaClient/GetSecret"); err != nil {
		t.Fatalf("locked daemon should defer to handlers, got: %v", err)
	}
}

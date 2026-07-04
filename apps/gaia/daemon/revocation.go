package daemon

import (
	"context"
	"strings"

	gaialog "github.com/stain-win/gaia/apps/gaia/log"
	"go.etcd.io/bbolt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// clientServicePrefix is the fully-qualified gRPC method prefix for the
// application-facing GaiaClient service.
const clientServicePrefix = "/gaia.GaiaClient/"

// checkPeerRevocation rejects GaiaClient calls from clients whose registration
// has been revoked. Revocation flips a status flag in the database rather than
// invalidating the certificate at the TLS layer, so without this check a revoked
// client's certificate would still pass the mTLS handshake and reach any handler
// that forgets to re-verify status. Enforcing it here, before any handler runs,
// closes that gap for the whole service.
//
// Certificates whose CN is not a registered client (e.g. the admin CLI cert)
// pass through: the handlers already reject unknown clients, and the GaiaAdmin
// service is gated separately by authorizeAdmin.
func (d *Daemon) checkPeerRevocation(ctx context.Context, fullMethod string) error {
	if !strings.HasPrefix(fullMethod, clientServicePrefix) {
		return nil
	}

	clientName, err := getClientIdentity(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, "a client certificate is required")
	}

	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	// While locked there is no database to consult; handlers report the locked
	// state with FailedPrecondition, which does not leak secret material.
	if d.isLocked || d.db == nil {
		return nil
	}

	var revoked bool
	viewErr := d.db.View(func(tx *bbolt.Tx) error {
		client, _, err := d.findClientByName(tx, clientName)
		if err == nil && client.Status == ClientStatusRevoked {
			revoked = true
		}
		return nil
	})
	if viewErr != nil {
		return status.Error(codes.Internal, "failed to verify client status")
	}

	if revoked {
		gaialog.Get().Warn("request rejected: client is revoked",
			"method", fullMethod,
			"client_name", clientName)
		return status.Error(codes.PermissionDenied, "client has been revoked")
	}

	return nil
}

/**
 * gaiatest — Test helpers for starting an ephemeral Gaia daemon in JS/TS tests.
 *
 * STATUS: Stub — full implementation coming in a future release.
 *         Use the Go gaiatest package (libs/go/gaiatest) for integration tests today.
 *
 * Planned API:
 *
 * ```ts
 * import { GaiaTestServer } from '@stain-win/gaia-client/gaiatest';
 *
 * describe('my feature', () => {
 *   let server: GaiaTestServer;
 *
 *   beforeAll(async () => {
 *     server = await GaiaTestServer.start();
 *   });
 *
 *   afterAll(() => server.stop());
 *
 *   it('can read a secret', async () => {
 *     const client = server.createClient();
 *     const secret = await client.getSecret({ namespace: 'app', id: 'key' });
 *     expect(secret.value).toBe('value');
 *   });
 * });
 * ```
 *
 * Requirements (when implemented):
 *  - `gaia` binary in PATH or GAIA_BIN env var
 *  - Node.js 18+
 */

export interface GaiaTestServerOptions {
  /** Path to the gaia binary. Defaults to GAIA_BIN env var or PATH lookup. */
  bin?: string;
  /** Master passphrase for the ephemeral daemon. Defaults to 'gaiatest-dev'. */
  passphrase?: string;
}

/**
 * GaiaTestServer manages an ephemeral Gaia daemon subprocess for testing.
 * @stub Full implementation coming soon.
 */
export class GaiaTestServer {
  /** gRPC address of the running daemon, e.g. "127.0.0.1:50123". */
  readonly addr: string = '';
  /** Directory containing ca.crt, gaia_client.crt, gaia_client.key. */
  readonly certDir: string = '';

  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private constructor() {}

  /**
   * Start an ephemeral Gaia daemon.
   * @stub Not yet implemented — throws NotImplementedError.
   */
  static async start(_opts?: GaiaTestServerOptions): Promise<GaiaTestServer> {
    throw new Error(
      'GaiaTestServer.start() is not yet implemented. ' +
        'Use the Go gaiatest package (libs/go/gaiatest) for integration tests.',
    );
  }

  /**
   * Stop the daemon and clean up all temporary files.
   * @stub No-op until implemented.
   */
  async stop(): Promise<void> {
    // no-op stub
  }
}

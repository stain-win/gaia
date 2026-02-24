import { createClient } from "@stain-win/gaia-client";
import * as path from "path";

// Module-level cache to ensure we only instantiate one client per Node execution
let clientInstance: Awaited<ReturnType<typeof createClient>> | null = null;
let clientStatus: string = "uninitialized";

export async function getGaiaClient() {
  if (clientInstance) {
    return { client: clientInstance, status: clientStatus };
  }

  const address = process.env.GAIA_ADDRESS || "localhost:50051";
  console.log(`[Gaia] Attempting to connect to ${address}`);

  try {
    clientInstance = await createClient({
      address,
      caCertFile: process.env.GAIA_CA_CERT,
      clientCertFile: process.env.GAIA_CLIENT_CERT,
      clientKeyFile: process.env.GAIA_CLIENT_KEY,
      hostOverride: process.env.GAIA_HOST_OVERRIDE || "localhost",
      protoPath: process.env.NODE_ENV === "production" 
        ? path.join(process.cwd(), "node_modules", "@stain-win/gaia-client", "proto", "gaia-client.proto")
        : undefined,
    });

    // The GAIA JS Client doesn't expose a getStatus method currently.
    // We assume if connect() succeeds during createClient, it's ready.
    clientStatus = "unlocked";
    console.log(`[Gaia] Connected successfully. Status: ${clientStatus}`);
    
    return { client: clientInstance, status: clientStatus };
  } catch (error) {
    console.error("[Gaia] Failed to create client:", error);
    clientStatus = "error";
    return { client: null, status: clientStatus, error: (error as Error).message };
  }
}

export async function getWebSecrets() {
  const { client, status, error } = await getGaiaClient();
  
  if (!client) {
    return { success: false, error: error || "Failed to initialize client", secrets: null };
  }

  if (status === "locked") {
    return { success: false, error: "Gaia Daemon is Locked", secrets: null };
  }

  if (status !== "unlocked") {
    return { success: false, error: `Gaia Daemon has unknown status: ${status}`, secrets: null };
  }

  console.log("[Gaia] Daemon is unlocked, fetching secrets list");

  try {
    const secrets = await client.listSecrets();
    return { success: true, secrets, error: null };
  } catch (err) {
    console.error("[Gaia] Failed to list secrets:", err);
    return { success: false, error: (err as Error).message, secrets: null };
  }
}

export async function loadAndGetEnv() {
  const { client, status, error } = await getGaiaClient();
  
  if (!client) {
    return { success: false, error: error || "Failed to initialize client", loadedEnv: null };
  }
  if (status !== "unlocked") {
    return { success: false, error: `Gaia Daemon has unknown status: ${status}`, loadedEnv: null };
  }

  try {
    // Load into process.env with a specific prefix so we can cleanly extract and show them
    await client.loadEnv({ prefix: "PLAYGROUND", useNamespace: true });
    
    // Extract only the keys we just loaded
    const loadedEnv: Record<string, string> = {};
    for (const key of Object.keys(process.env)) {
      if (key.startsWith("PLAYGROUND_")) {
        loadedEnv[key] = process.env[key] as string;
      }
    }
    
    return { success: true, loadedEnv, error: null };
  } catch (err) {
    console.error("[Gaia] Failed to load env:", err);
    return { success: false, error: (err as Error).message, loadedEnv: null };
  }
}

import { getWebSecrets, loadAndGetEnv } from "./gaia-api";
import { FaLock, FaUnlock, FaServer, FaKey, FaEnvira } from "react-icons/fa";

export const revalidate = 0; // Disable static caching

export default async function Home() {
  const { success, secrets, error } = await getWebSecrets();
  const { success: envSuccess, loadedEnv, error: envError } = await loadAndGetEnv();

  return (
    <div className="max-w-4xl mx-auto p-8 pt-16">
      <header className="mb-12 text-center">
        <h1 className="text-4xl font-bold mb-4 font-sans tracking-tight">
          <span className="text-transparent bg-clip-text bg-gradient-to-r from-cyan-400 to-emerald-400">
            Gaia
          </span>{" "}
          Playground
        </h1>
        <p className="text-gray-400">
          A demonstration of retrieving environment context dynamically.
        </p>
      </header>

      <section className="glass-card p-6 mb-8 rounded-xl border border-white/5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold flex items-center gap-2">
            <FaServer className="text-cyan-400" /> Daemon Status
          </h2>
          <div>
            {!success ? (
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-sm font-medium bg-red-500/10 text-red-400 border border-red-500/20">
                <FaLock /> Locked / Error
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-sm font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                <FaUnlock /> Unlocked
              </span>
            )}
          </div>
        </div>

        {error && (
          <div className="p-4 rounded bg-red-500/10 border border-red-500/20 text-red-400 mb-4 font-mono text-sm">
            {error}
          </div>
        )}
      </section>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        <section>
          <h2 className="text-2xl font-semibold mb-6 flex items-center gap-2">
            <FaKey className="text-emerald-400" /> Active Secrets
          </h2>

          {!success || !secrets || Object.keys(secrets).length === 0 ? (
            <div className="glass-card p-8 text-center rounded-xl border border-white/5">
              <p className="text-gray-500 mb-2">No secrets found in the daemon.</p>
              <p className="text-sm text-gray-400">
                Try running <code className="px-1.5 py-0.5 rounded bg-black/30 font-mono text-cyan-400">docker-compose exec gaia gaia secrets import /tmp/test_secrets.json</code>
              </p>
            </div>
          ) : (
            <div className="space-y-6">
              {Object.entries(secrets).map(([namespace, kv]) => (
                <div key={namespace} className="glass-card rounded-xl overflow-hidden border border-white/5">
                  <div className="bg-white/5 px-6 py-3 border-b border-white/5">
                    <h3 className="font-mono text-sm uppercase tracking-wider text-cyan-400">
                      Namespace: {namespace}
                    </h3>
                  </div>
                  <div className="divide-y divide-white/5">
                    {Object.entries(kv as any).map(([key, value]) => (
                      <div key={key} className="px-6 py-4 flex flex-col sm:flex-row sm:items-center justify-between secret-row hover:bg-white/5 transition-colors">
                        <div className="font-mono text-gray-300 font-medium">{key}</div>
                        <div className="font-mono text-emerald-400 text-sm mt-1 sm:mt-0 break-all bg-black/20 px-3 py-1 rounded">
                          {value as string}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        <section>
          <h2 className="text-2xl font-semibold mb-6 flex items-center gap-2">
            <FaEnvira className="text-emerald-400" /> LOADED TO ENV
          </h2>
          
          <div className="glass-card rounded-xl overflow-hidden border border-white/5">
            <div className="bg-white/5 px-6 py-3 border-b border-white/5 flex items-center justify-between">
              <h3 className="font-mono text-sm uppercase tracking-wider text-cyan-400">
                process.env (Prefixed)
              </h3>
            </div>
            <div className="p-4 bg-black/40 overflow-x-auto">
              {envError ? (
                <div className="text-red-400 font-mono text-sm">{envError}</div>
              ) : !envSuccess || !loadedEnv || Object.keys(loadedEnv).length === 0 ? (
                 <div className="text-gray-500 font-mono text-sm">No new environment variables loaded.</div>
              ) : (
                <pre className="font-mono text-sm text-emerald-300">
                  {JSON.stringify(loadedEnv, null, 2)}
                </pre>
              )}
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}

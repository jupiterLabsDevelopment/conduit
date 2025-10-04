import { FormEvent, useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { useAuth } from "../context/AuthContext";
import type { ServerListItem } from "../lib/api";

interface CreateServerState {
  name: string;
  description: string;
  agentToken?: string;
  error?: string | null;
}

const ServersPage = () => {
  const { api, user } = useAuth();
  const [servers, setServers] = useState<ServerListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createState, setCreateState] = useState<CreateServerState>({ name: "", description: "" });
  const [creating, setCreating] = useState(false);

  const loadServers = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listServers();
      setServers(data);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void loadServers();
  }, [loadServers]);

  const handleCreate = async (evt: FormEvent<HTMLFormElement>) => {
    evt.preventDefault();
    if (!createState.name.trim()) {
      setCreateState((prev) => ({ ...prev, error: "Server name is required" }));
      return;
    }
    setCreating(true);
    setCreateState((prev) => ({ ...prev, error: null }));
    try {
      const res = await api.createServer({
        name: createState.name.trim(),
        description: createState.description.trim() || undefined,
      });
      setCreateState({ name: "", description: "", agentToken: res.agent_token });
      await loadServers();
    } catch (err) {
      setCreateState((prev) => ({ ...prev, error: (err as Error).message }));
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="space-y-10">
      <section className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight text-slate-100">Servers</h1>
          <p className="text-sm text-slate-400">Manage connected Minecraft servers and their agents.</p>
        </div>
      </section>

      {user?.role === "owner" ? (
        <section className="rounded-xl border border-slate-800 bg-slate-950/60 p-6 shadow-md shadow-slate-950/20">
          <h2 className="text-lg font-semibold text-cyan">Register a new server</h2>
          <p className="mt-1 text-xs text-slate-400">Issue an agent token and track its connection state.</p>
          <form className="mt-4 grid gap-4 sm:grid-cols-2" onSubmit={handleCreate}>
            <div className="sm:col-span-1">
              <label className="text-xs font-semibold uppercase tracking-wide text-slate-400" htmlFor="server-name">
                Name
              </label>
              <input
                id="server-name"
                required
                value={createState.name}
                onChange={(evt) => setCreateState((prev) => ({ ...prev, name: evt.target.value }))}
                className="mt-1 w-full rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-100 focus:border-sky focus:outline-none focus:ring-2 focus:ring-sky"
                placeholder="Survival Realm"
              />
            </div>
            <div className="sm:col-span-1">
              <label className="text-xs font-semibold uppercase tracking-wide text-slate-400" htmlFor="server-description">
                Description
              </label>
              <input
                id="server-description"
                value={createState.description}
                onChange={(evt) => setCreateState((prev) => ({ ...prev, description: evt.target.value }))}
                className="mt-1 w-full rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-100 focus:border-sky focus:outline-none focus:ring-2 focus:ring-sky"
                placeholder="Primary SMP server"
              />
            </div>
            {createState.error ? (
              <p className="sm:col-span-2 text-sm text-rose-400">{createState.error}</p>
            ) : null}
            <div className="sm:col-span-2 flex items-center justify-between">
              <button
                type="submit"
                disabled={creating}
                className="rounded-md bg-cyan px-4 py-2 text-xs font-semibold uppercase tracking-wide text-slate-900 hover:bg-sky disabled:opacity-60"
              >
                {creating ? "Issuing token" : "Create server"}
              </button>
              {createState.agentToken ? (
                <div className="text-xs text-slate-300">
                  Agent token:
                  <code className="ml-2 rounded bg-slate-900 px-2 py-1 font-mono text-[11px] text-cyan">
                    {createState.agentToken}
                  </code>
                </div>
              ) : null}
            </div>
          </form>
        </section>
      ) : null}

      <section>
        {loading ? (
          <div className="rounded-xl border border-slate-900 bg-slate-950/60 p-10 text-center text-slate-300">
            Loading servers…
          </div>
        ) : error ? (
          <div className="rounded-xl border border-rose-900/50 bg-rose-950/40 p-6 text-sm text-rose-200">{error}</div>
        ) : servers.length === 0 ? (
          <div className="rounded-xl border border-slate-900 bg-slate-950/50 p-10 text-center text-slate-300">
            No servers yet. Create one to get started.
          </div>
        ) : (
          <div className="grid gap-6 md:grid-cols-2">
            {servers.map((server) => (
              <Link
                key={server.id}
                to={`/servers/${server.id}`}
                className="group rounded-xl border border-slate-800 bg-slate-950/60 p-6 transition hover:border-sky hover:bg-slate-900/80"
              >
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="text-lg font-semibold text-slate-100 group-hover:text-cyan">{server.name}</h3>
                    {server.description ? <p className="text-sm text-slate-400">{server.description}</p> : null}
                  </div>
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs font-semibold uppercase tracking-wide ${
                      server.connected ? "bg-emerald-500/20 text-emerald-300" : "bg-slate-800 text-slate-300"
                    }`}
                  >
                    {server.connected ? "Online" : "Offline"}
                  </span>
                </div>
                <p className="mt-4 text-xs text-slate-500">
                  Last sync: {server.connected_at ? new Date(server.connected_at).toLocaleString() : "never"}
                </p>
              </Link>
            ))}
          </div>
        )}
      </section>
    </div>
  );
};

export default ServersPage;

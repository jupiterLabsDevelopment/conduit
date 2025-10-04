import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "../context/AuthContext";
import type { ApiKeySummary } from "../lib/api";

const formatTimestamp = (value: string): string => new Date(value).toLocaleString();

const ApiKeysPage = () => {
  const { api, user } = useAuth();
  const [keys, setKeys] = useState<ApiKeySummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const [createdSecret, setCreatedSecret] = useState<{ name: string; value: string } | null>(null);

  const isOwner = user?.role === "owner";

  const loadKeys = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await api.listApiKeys();
      setKeys(list);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void loadKeys();
  }, [loadKeys]);

  const handleCreate = useCallback(async (evt: FormEvent<HTMLFormElement>) => {
    evt.preventDefault();
    if (!newName.trim()) {
      setCreateError("Key name is required");
      return;
    }
    setCreating(true);
    setCreateError(null);
    try {
      const created = await api.createApiKey(newName.trim());
      setKeys((prev) => [created, ...prev]);
      setCreatedSecret({ name: created.name, value: created.secret });
      setNewName("");
    } catch (err) {
      setCreateError((err as Error).message);
    } finally {
      setCreating(false);
    }
  }, [api, newName]);

  const handleDelete = useCallback(async (key: ApiKeySummary) => {
    if (!window.confirm(`Revoke API key "${key.name}"? This cannot be undone.`)) {
      return;
    }
    try {
      await api.deleteApiKey(key.id);
      setKeys((prev) => prev.filter((item) => item.id !== key.id));
    } catch (err) {
      window.alert((err as Error).message);
    }
  }, [api]);

  const sortedKeys = useMemo(
    () => keys.slice().sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()),
    [keys],
  );

  if (!isOwner) {
    return (
      <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-8 text-center text-sm text-slate-300">
        API keys can only be managed by owners.
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <header className="space-y-2">
        <h1 className="text-3xl font-semibold tracking-tight text-slate-100">API keys</h1>
        <p className="text-sm text-slate-400">Issue and revoke personal API keys for automation.
        </p>
      </header>

      <section className="rounded-xl border border-slate-800 bg-slate-950/60 p-6 shadow-md shadow-slate-950/20">
        <h2 className="text-lg font-semibold text-cyan">Create new key</h2>
        <p className="text-xs text-slate-400">Keys inherit your permissions. Store generated secrets securely.</p>
        <form className="mt-4 flex flex-col gap-4 sm:flex-row sm:items-end" onSubmit={handleCreate}>
          <label className="flex w-full flex-col text-xs font-semibold uppercase tracking-wide text-slate-400 sm:w-80">
            Key name
            <input
              className="mt-1 rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-100 focus:border-sky focus:outline-none focus:ring-2 focus:ring-sky"
              value={newName}
              onChange={(evt) => setNewName(evt.target.value)}
              placeholder="CI runner"
              required
            />
          </label>
          <button
            type="submit"
            disabled={creating}
            className="rounded-md bg-cyan px-4 py-2 text-xs font-semibold uppercase tracking-wide text-slate-900 hover:bg-sky disabled:opacity-60"
          >
            {creating ? "Issuing" : "Create key"}
          </button>
        </form>
        {createError ? <p className="mt-3 text-sm text-rose-300">{createError}</p> : null}
        {createdSecret ? (
          <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900/60 p-4 text-sm text-slate-200">
            <p className="text-xs uppercase tracking-wide text-slate-400">Secret for {createdSecret.name}</p>
            <code className="mt-2 block break-all rounded bg-slate-950 px-3 py-2 font-mono text-xs text-cyan">
              {createdSecret.value}
            </code>
            <p className="mt-2 text-xs text-slate-500">Store this secret now. You won&apos;t be able to view it again.</p>
          </div>
        ) : null}
      </section>

      <section className="rounded-xl border border-slate-800 bg-slate-950/60">
        <div className="flex items-center justify-between border-b border-slate-800 px-6 py-4">
          <div>
            <h2 className="text-lg font-semibold text-cyan">Active keys</h2>
            <p className="text-xs text-slate-400">Revoke unused keys to keep access tight.</p>
          </div>
          <button
            type="button"
            onClick={() => loadKeys()}
            className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
          >
            Refresh
          </button>
        </div>
        {error ? (
          <div className="px-6 py-4 text-sm text-rose-300">{error}</div>
        ) : null}
        {loading ? (
          <div className="px-6 py-6 text-sm text-slate-400">Loading keys…</div>
        ) : sortedKeys.length === 0 ? (
          <div className="px-6 py-6 text-sm text-slate-400">No API keys issued yet.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-slate-800 text-sm">
              <thead className="bg-slate-950/80 text-xs uppercase tracking-wide text-slate-400">
                <tr>
                  <th className="px-6 py-3 text-left">Name</th>
                  <th className="px-6 py-3 text-left">Created</th>
                  <th className="px-6 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {sortedKeys.map((key) => (
                  <tr key={key.id}>
                    <td className="px-6 py-3 font-medium text-slate-100">{key.name}</td>
                    <td className="px-6 py-3 text-slate-300">{formatTimestamp(key.created_at)}</td>
                    <td className="px-6 py-3 text-right">
                      <button
                        type="button"
                        onClick={() => handleDelete(key)}
                        className="rounded-md border border-rose-700 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-rose-200 transition hover:bg-rose-700/10"
                      >
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
};

export default ApiKeysPage;

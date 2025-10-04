import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { EventStream } from "../components/EventStream";
import { RpcActionCard } from "../components/RpcActionCard";
import { useAuth } from "../context/AuthContext";
import { useServerEvents } from "../hooks/useServerEvents";
import type { ApplyPresetResponse, AuditLogEntry, GameRulePreset, PresetApplicationResult, ServerDetail } from "../lib/api";

type TabKey = "overview" | "players" | "gamerules" | "settings" | "audit";

interface AllowlistEntry {
  name: string;
  uuid?: string;
}

interface OperatorEntry {
  name: string;
  permission?: string;
}

interface RuleEntry {
  name: string;
  value: unknown;
}

interface SettingEntry {
  name: string;
  value: unknown;
}

const tabs: Array<{ id: TabKey; label: string }> = [
  { id: "overview", label: "Overview" },
  { id: "players", label: "Players" },
  { id: "gamerules", label: "Game rules" },
  { id: "settings", label: "Settings" },
  { id: "audit", label: "Audit log" },
];

const formatTimestamp = (value: string | null | undefined): string => {
  if (!value) {
    return "never";
  }
  return new Date(value).toLocaleString();
};

const displayValue = (value: unknown): string => {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (value === null || value === undefined) {
    return "";
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
};

const coerceInputValue = (input: string, current: unknown): unknown => {
  const trimmed = input.trim();
  if (typeof current === "boolean") {
    return trimmed.toLowerCase() === "true";
  }
  if (typeof current === "number") {
    const num = Number(trimmed);
    return Number.isNaN(num) ? current : num;
  }
  if (trimmed.toLowerCase() === "true") {
    return true;
  }
  if (trimmed.toLowerCase() === "false") {
    return false;
  }
  const num = Number(trimmed);
  if (!Number.isNaN(num) && trimmed !== "") {
    return num;
  }
  return trimmed;
};

const ServerDetailPage = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { api, user } = useAuth();

  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [server, setServer] = useState<ServerDetail | null>(null);
  const [schema, setSchema] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [allowlist, setAllowlist] = useState<AllowlistEntry[]>([]);
  const [operators, setOperators] = useState<OperatorEntry[]>([]);
  const [playersLoading, setPlayersLoading] = useState(false);
  const [playersError, setPlayersError] = useState<string | null>(null);
  const [playersLoaded, setPlayersLoaded] = useState(false);

  const [gameRules, setGameRules] = useState<RuleEntry[]>([]);
  const [gameRulesLoading, setGameRulesLoading] = useState(false);
  const [gameRulesError, setGameRulesError] = useState<string | null>(null);
  const [gameRulesLoaded, setGameRulesLoaded] = useState(false);

  const [gameRulePresets, setGameRulePresets] = useState<GameRulePreset[]>([]);
  const [gameRulePresetsLoading, setGameRulePresetsLoading] = useState(false);
  const [gameRulePresetsLoaded, setGameRulePresetsLoaded] = useState(false);
  const [gameRulePresetsError, setGameRulePresetsError] = useState<string | null>(null);
  const [selectedPresetKey, setSelectedPresetKey] = useState<string>("");
  const [presetApplying, setPresetApplying] = useState(false);
  const [presetApplyError, setPresetApplyError] = useState<string | null>(null);
  const [presetApplyResult, setPresetApplyResult] = useState<ApplyPresetResponse | null>(null);

  const [settings, setSettings] = useState<SettingEntry[]>([]);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [settingsError, setSettingsError] = useState<string | null>(null);
  const [settingsLoaded, setSettingsLoaded] = useState(false);

  const [auditLogs, setAuditLogs] = useState<AuditLogEntry[]>([]);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditError, setAuditError] = useState<string | null>(null);
  const [auditLoaded, setAuditLoaded] = useState(false);
  const [auditExporting, setAuditExporting] = useState(false);
  const [auditExportError, setAuditExportError] = useState<string | null>(null);

  const events = useServerEvents(id);

  const selectedPreset = useMemo(() => gameRulePresets.find((preset) => preset.key === selectedPresetKey) ?? null, [gameRulePresets, selectedPresetKey]);
  const presetResults: PresetApplicationResult[] = presetApplyResult?.results ?? [];

  const loadServer = useCallback(async () => {
    if (!id) {
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const detail = await api.getServer(id);
      setServer(detail);
      const nextSchema = await api.getServerSchema(id);
      setSchema(nextSchema);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, [api, id]);

  useEffect(() => {
    void loadServer();
  }, [loadServer]);

  useEffect(() => {
    if (!id) {
      navigate("/servers", { replace: true });
    }
  }, [id, navigate]);

  const canModerate = user?.role === "owner" || user?.role === "moderator";
  const canOwner = user?.role === "owner";

  const callRpc = useCallback(async (method: string, params: unknown) => {
    if (!id) {
      throw new Error("Server id missing");
    }
    await api.callServerRpc(id, method, params);
  }, [api, id]);

  const fetchPlayers = useCallback(async () => {
    if (!id) {
      return;
    }
    setPlayersLoading(true);
    setPlayersError(null);
    try {
      const allowResp = await api.callServerRpc<{ allowed?: AllowlistEntry[] }>(id, "minecraft:allowlist/list", []);
      const opsResp = await api.callServerRpc<{ operators?: OperatorEntry[] }>(id, "minecraft:operators/list", []);
      setAllowlist(Array.isArray(allowResp?.allowed) ? allowResp.allowed : []);
      setOperators(Array.isArray(opsResp?.operators) ? opsResp.operators : []);
    } catch (err) {
      setPlayersError((err as Error).message);
    } finally {
      setPlayersLoading(false);
    }
  }, [api, id]);

  const fetchGameRules = useCallback(async () => {
    if (!id) {
      return;
    }
    setGameRulesLoading(true);
    setGameRulesError(null);
    try {
      const resp = await api.callServerRpc<{ rules?: RuleEntry[] }>(id, "minecraft:gamerule/list", []);
      const list = Array.isArray(resp?.rules) ? resp?.rules ?? [] : [];
      setGameRules(list);
    } catch (err) {
      setGameRulesError((err as Error).message);
    } finally {
      setGameRulesLoading(false);
    }
  }, [api, id]);

  const fetchGameRulePresets = useCallback(async () => {
    setGameRulePresetsLoading(true);
    setGameRulePresetsError(null);
    try {
      const presets = await api.listGameRulePresets();
      setGameRulePresets(presets);
      setSelectedPresetKey((prev) => (prev ? prev : presets[0]?.key ?? ""));
    } catch (err) {
      setGameRulePresetsError((err as Error).message);
    } finally {
      setGameRulePresetsLoading(false);
      setGameRulePresetsLoaded(true);
    }
  }, [api]);

  const fetchSettings = useCallback(async () => {
    if (!id) {
      return;
    }
    setSettingsLoading(true);
    setSettingsError(null);
    try {
      const resp = await api.callServerRpc<{ settings?: SettingEntry[] | Record<string, unknown> }>(id, "minecraft:settings/list", []);
      if (Array.isArray(resp?.settings)) {
        setSettings(resp?.settings ?? []);
      } else if (resp && resp.settings && typeof resp.settings === "object") {
        const entries = Object.entries(resp.settings).map(([name, value]) => ({ name, value }));
        setSettings(entries);
      } else {
        setSettings([]);
      }
    } catch (err) {
      setSettingsError((err as Error).message);
    } finally {
      setSettingsLoading(false);
    }
  }, [api, id]);

  const fetchAuditLogs = useCallback(async () => {
    if (!id) {
      return;
    }
    setAuditLoading(true);
    setAuditError(null);
    setAuditExportError(null);
    try {
      const logs = await api.listAuditLogs(id, 100);
      setAuditLogs(logs);
    } catch (err) {
      setAuditError((err as Error).message);
    } finally {
      setAuditLoading(false);
    }
  }, [api, id]);

  const handlePresetApply = useCallback(async () => {
    if (!id) {
      return;
    }
    if (!selectedPresetKey) {
      setPresetApplyError("Select a preset before applying");
      return;
    }

    setPresetApplying(true);
    setPresetApplyError(null);
    setPresetApplyResult(null);
    try {
      const response = await api.applyGameRulePreset(id, selectedPresetKey);
      setPresetApplyResult(response);
      await Promise.all([fetchGameRules(), fetchSettings()]);
    } catch (err) {
      setPresetApplyError((err as Error).message);
    } finally {
      setPresetApplying(false);
    }
  }, [api, fetchGameRules, fetchSettings, id, selectedPresetKey]);

  const handleAuditExport = useCallback(async () => {
    if (!id) {
      return;
    }
    setAuditExportError(null);
    setAuditExporting(true);
    try {
      const csv = await api.exportAuditLogs(id, { limit: 5000 });
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `server-${id}-audit.csv`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      setAuditExportError((err as Error).message);
    } finally {
      setAuditExporting(false);
    }
  }, [api, id]);

  useEffect(() => {
    if (activeTab === "players" && !playersLoaded) {
      setPlayersLoaded(true);
      void fetchPlayers();
    }
    if (activeTab === "gamerules" && !gameRulesLoaded) {
      setGameRulesLoaded(true);
      void fetchGameRules();
    }
    if (activeTab === "gamerules" && !gameRulePresetsLoaded) {
      void fetchGameRulePresets();
    }
    if (activeTab === "settings" && !settingsLoaded) {
      setSettingsLoaded(true);
      void fetchSettings();
    }
    if (activeTab === "audit" && !auditLoaded) {
      setAuditLoaded(true);
      void fetchAuditLogs();
    }
  }, [activeTab, auditLoaded, fetchAuditLogs, fetchGameRules, fetchPlayers, fetchSettings, gameRulesLoaded, playersLoaded, settingsLoaded]);

  const handleAllowlistAdd = useCallback(async (values: Record<string, string>) => {
    const username = values.username?.trim();
    if (!username) {
      throw new Error("Player username required");
    }
    await callRpc("minecraft:allowlist/add", [[{ name: username }]]);
    await fetchPlayers();
  }, [callRpc, fetchPlayers]);

  const handleAllowlistRemove = useCallback(async (values: Record<string, string>) => {
    const username = values.username?.trim();
    if (!username) {
      throw new Error("Player username required");
    }
    await callRpc("minecraft:allowlist/remove", [[{ name: username }]]);
    await fetchPlayers();
  }, [callRpc, fetchPlayers]);

  const handleOperatorAdd = useCallback(async (values: Record<string, string>) => {
    const username = values.username?.trim();
    if (!username) {
      throw new Error("Player username required");
    }
    await callRpc("minecraft:operators/add", [[{ name: username }]]);
    await fetchPlayers();
  }, [callRpc, fetchPlayers]);

  const handleOperatorRemove = useCallback(async (values: Record<string, string>) => {
    const username = values.username?.trim();
    if (!username) {
      throw new Error("Player username required");
    }
    await callRpc("minecraft:operators/remove", [[{ name: username }]]);
    await fetchPlayers();
  }, [callRpc, fetchPlayers]);

  const handleServerSave = useCallback(async () => {
    await callRpc("minecraft:server/save", []);
    await loadServer();
  }, [callRpc, loadServer]);

  const handleServerStop = useCallback(async () => {
    if (!window.confirm("Stop the server? Players will be disconnected.")) {
      return;
    }
    await callRpc("minecraft:server/stop", []);
    await loadServer();
  }, [callRpc, loadServer]);

  const handleGameRuleUpdate = useCallback(async (rule: RuleEntry) => {
    const next = window.prompt(`Set value for ${rule.name}`, displayValue(rule.value));
    if (next == null) {
      return;
    }
    try {
      await callRpc("minecraft:gamerule/set", [rule.name, coerceInputValue(next, rule.value)]);
      await fetchGameRules();
    } catch (err) {
      window.alert((err as Error).message);
    }
  }, [callRpc, fetchGameRules]);

  const handleSettingUpdate = useCallback(async (setting: SettingEntry) => {
    const next = window.prompt(`Set value for ${setting.name}`, displayValue(setting.value));
    if (next == null) {
      return;
    }
    try {
      await callRpc("minecraft:settings/set", [setting.name, coerceInputValue(next, setting.value)]);
      await fetchSettings();
    } catch (err) {
      window.alert((err as Error).message);
    }
  }, [callRpc, fetchSettings]);

  const statusBadge = useMemo(() => {
    if (!server) {
      return null;
    }
    const baseClass = "rounded-full px-2 py-0.5 text-xs font-semibold uppercase tracking-wide";
    if (server.connected) {
      return <span className={`${baseClass} bg-emerald-500/20 text-emerald-300`}>Online</span>;
    }
    return <span className={`${baseClass} bg-slate-800 text-slate-300`}>Offline</span>;
  }, [server]);

  const renderPlayersTab = () => (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-cyan">Allowlist & operators</h2>
          <p className="text-xs text-slate-400">Review current access lists and issue updates.</p>
        </div>
        <button
          type="button"
          onClick={() => fetchPlayers()}
          className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
        >
          Refresh
        </button>
      </div>
      {playersError ? (
        <div className="rounded-lg border border-rose-900/50 bg-rose-950/40 px-4 py-3 text-sm text-rose-200">{playersError}</div>
      ) : null}
      <div className="grid gap-6 xl:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-5">
          <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-300">Allowlist</h3>
          {playersLoading && allowlist.length === 0 ? (
            <p className="mt-4 text-sm text-slate-400">Loading allowlist…</p>
          ) : allowlist.length === 0 ? (
            <p className="mt-4 text-sm text-slate-400">No players are currently allowlisted.</p>
          ) : (
            <ul className="mt-4 space-y-2 text-sm text-slate-100">
              {allowlist.map((item) => (
                <li key={item.name} className="flex items-center justify-between rounded border border-slate-800/60 bg-slate-900/50 px-3 py-2">
                  <span>{item.name}</span>
                  {item.uuid ? <span className="text-xs text-slate-500">{item.uuid}</span> : null}
                </li>
              ))}
            </ul>
          )}
        </div>
        <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-5">
          <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-300">Operators</h3>
          {playersLoading && operators.length === 0 ? (
            <p className="mt-4 text-sm text-slate-400">Loading operators…</p>
          ) : operators.length === 0 ? (
            <p className="mt-4 text-sm text-slate-400">No operators configured.</p>
          ) : (
            <ul className="mt-4 space-y-2 text-sm text-slate-100">
              {operators.map((item) => (
                <li key={item.name} className="flex items-center justify-between rounded border border-slate-800/60 bg-slate-900/50 px-3 py-2">
                  <span>{item.name}</span>
                  {item.permission ? <span className="text-xs text-slate-500">{item.permission}</span> : null}
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
      <div className="grid gap-6 md:grid-cols-2">
        <RpcActionCard
          title="Add to allowlist"
          description="Allow a player to join the server."
          buttonLabel="Add player"
          disabled={!canModerate || !server?.connected}
          onSubmit={handleAllowlistAdd}
          fields={[{ name: "username", label: "Player username", placeholder: "Notch", required: true }]}
        />
        <RpcActionCard
          title="Remove from allowlist"
          description="Revoke access for a player."
          buttonLabel="Remove player"
          disabled={!canModerate || !server?.connected}
          onSubmit={handleAllowlistRemove}
          fields={[{ name: "username", label: "Player username", placeholder: "Notch", required: true }]}
        />
        <RpcActionCard
          title="Promote to operator"
          description="Grant operator privileges to a player."
          buttonLabel="Promote"
          disabled={!canModerate || !server?.connected}
          onSubmit={handleOperatorAdd}
          fields={[{ name: "username", label: "Player username", placeholder: "Player", required: true }]}
        />
        <RpcActionCard
          title="Remove operator"
          description="Remove operator status from a player."
          buttonLabel="Demote"
          disabled={!canModerate || !server?.connected}
          onSubmit={handleOperatorRemove}
          fields={[{ name: "username", label: "Player username", placeholder: "Player", required: true }]}
        />
      </div>
    </div>
  );

  const renderGameRulesTab = () => (
    <div className="space-y-6">
      {canModerate ? (
        <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-5">
          <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
            <div>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-cyan">Bulk presets</h3>
              <p className="text-xs text-slate-400">Apply curated changes across multiple game rules and settings.</p>
            </div>
            <div className="flex flex-col gap-2 md:flex-row md:items-center">
              <select
                value={selectedPresetKey}
                onChange={(evt) => {
                  setSelectedPresetKey(evt.target.value);
                  setPresetApplyError(null);
                  setPresetApplyResult(null);
                }}
                disabled={gameRulePresetsLoading || gameRulePresets.length === 0}
                className="min-w-[12rem] rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-100 focus:border-sky focus:outline-none focus:ring-2 focus:ring-sky disabled:cursor-not-allowed disabled:opacity-60"
              >
                {gameRulePresets.length === 0 ? <option value="">No presets available</option> : null}
                {gameRulePresets.map((preset) => (
                  <option key={preset.key} value={preset.key}>
                    {preset.label}
                  </option>
                ))}
              </select>
              <button
                type="button"
                onClick={() => {
                  void handlePresetApply();
                }}
                disabled={presetApplying || !selectedPresetKey || !server?.connected}
                className="rounded-md border border-slate-700 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky disabled:cursor-not-allowed disabled:opacity-60"
              >
                {presetApplying ? "Applying…" : "Apply preset"}
              </button>
            </div>
          </div>
          {gameRulePresetsLoading ? (
            <p className="mt-3 text-xs text-slate-400">Loading presets…</p>
          ) : null}
          {gameRulePresetsError ? (
            <div className="mt-3 rounded border border-rose-900/40 bg-rose-950/40 px-3 py-2 text-xs text-rose-200">{gameRulePresetsError}</div>
          ) : null}
          {selectedPreset ? (
            <div className="mt-4 space-y-3 text-xs text-slate-300">
              <p className="text-[11px] uppercase tracking-wide text-slate-400">{selectedPreset.description}</p>
              <div className="grid gap-4 md:grid-cols-2">
                <div>
                  <h4 className="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Game rules</h4>
                  {selectedPreset.game_rules && Object.keys(selectedPreset.game_rules).length > 0 ? (
                    <ul className="mt-2 space-y-1">
                      {Object.entries(selectedPreset.game_rules).map(([name, value]) => (
                        <li key={name} className="flex items-center justify-between rounded border border-slate-800/60 bg-slate-900/50 px-3 py-2">
                          <span className="font-medium text-slate-100">{name}</span>
                          <span className="text-slate-300">{displayValue(value)}</span>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="mt-2 text-slate-500">No game rule changes.</p>
                  )}
                </div>
                <div>
                  <h4 className="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Settings</h4>
                  {selectedPreset.settings && Object.keys(selectedPreset.settings).length > 0 ? (
                    <ul className="mt-2 space-y-1">
                      {Object.entries(selectedPreset.settings).map(([name, value]) => (
                        <li key={name} className="flex items-center justify-between rounded border border-slate-800/60 bg-slate-900/50 px-3 py-2">
                          <span className="font-medium text-slate-100">{name}</span>
                          <span className="text-slate-300">{displayValue(value)}</span>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="mt-2 text-slate-500">No setting changes.</p>
                  )}
                </div>
              </div>
            </div>
          ) : null}
          {presetApplyError ? (
            <div className="mt-3 rounded border border-rose-900/40 bg-rose-950/40 px-3 py-2 text-xs text-rose-200">{presetApplyError}</div>
          ) : null}
          {presetApplyResult ? (
            <div className="mt-4 space-y-3">
              <p className="text-xs text-slate-400">
                Applied {presetResults.filter((item) => item.status === "ok").length}/{presetResults.length} updates in {presetApplyResult.duration_ms} ms.
              </p>
              {presetResults.length > 0 ? (
                <div className="overflow-hidden rounded-lg border border-slate-800">
                  <table className="min-w-full divide-y divide-slate-800 text-xs">
                    <thead className="bg-slate-950/80 text-[11px] uppercase tracking-wide text-slate-400">
                      <tr>
                        <th className="px-3 py-2 text-left">Type</th>
                        <th className="px-3 py-2 text-left">Name</th>
                        <th className="px-3 py-2 text-left">Value</th>
                        <th className="px-3 py-2 text-left">Status</th>
                        <th className="px-3 py-2 text-left">Message</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60">
                      {presetResults.map((result, idx) => (
                        <tr key={`${result.type}-${result.name}-${idx}`}>
                          <td className="px-3 py-2 text-slate-300">{result.type}</td>
                          <td className="px-3 py-2 font-medium text-slate-100">{result.name}</td>
                          <td className="px-3 py-2 text-slate-200">{displayValue(result.value)}</td>
                          <td className="px-3 py-2">
                            <span
                              className={`rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${
                                result.status === "ok" ? "bg-emerald-500/20 text-emerald-300" : "bg-rose-500/20 text-rose-200"
                              }`}
                            >
                              {result.status}
                            </span>
                          </td>
                          <td className="px-3 py-2 text-slate-300">{result.message ?? ""}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}

      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-cyan">Game rules</h2>
          <p className="text-xs text-slate-400">Review and update configured game rules.</p>
        </div>
        <button
          type="button"
          onClick={() => fetchGameRules()}
          className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
        >
          Refresh
        </button>
      </div>
      {gameRulesError ? (
        <div className="rounded-lg border border-rose-900/50 bg-rose-950/40 px-4 py-3 text-sm text-rose-200">{gameRulesError}</div>
      ) : null}
      <div className="overflow-hidden rounded-xl border border-slate-800 bg-slate-950/60">
        <table className="min-w-full divide-y divide-slate-800 text-sm">
          <thead className="bg-slate-950/80 text-xs uppercase tracking-wide text-slate-400">
            <tr>
              <th className="px-4 py-3 text-left">Rule</th>
              <th className="px-4 py-3 text-left">Value</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60">
            {gameRulesLoading && gameRules.length === 0 ? (
              <tr>
                <td colSpan={3} className="px-4 py-4 text-center text-slate-400">
                  Loading game rules…
                </td>
              </tr>
            ) : gameRules.length === 0 ? (
              <tr>
                <td colSpan={3} className="px-4 py-4 text-center text-slate-400">
                  No game rules available.
                </td>
              </tr>
            ) : (
              gameRules.map((rule) => (
                <tr key={rule.name}>
                  <td className="px-4 py-3 font-medium text-slate-100">{rule.name}</td>
                  <td className="px-4 py-3 text-slate-200">{displayValue(rule.value)}</td>
                  <td className="px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={() => handleGameRuleUpdate(rule)}
                      className="rounded-md border border-slate-700 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
                      disabled={!canModerate || !server?.connected}
                    >
                      Update
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );

  const renderSettingsTab = () => (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-cyan">Server settings</h2>
          <p className="text-xs text-slate-400">Inspect settings surfaced by the agent and apply overrides.</p>
        </div>
        <button
          type="button"
          onClick={() => fetchSettings()}
          className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
        >
          Refresh
        </button>
      </div>
      {settingsError ? (
        <div className="rounded-lg border border-rose-900/50 bg-rose-950/40 px-4 py-3 text-sm text-rose-200">{settingsError}</div>
      ) : null}
      <div className="overflow-hidden rounded-xl border border-slate-800 bg-slate-950/60">
        <table className="min-w-full divide-y divide-slate-800 text-sm">
          <thead className="bg-slate-950/80 text-xs uppercase tracking-wide text-slate-400">
            <tr>
              <th className="px-4 py-3 text-left">Setting</th>
              <th className="px-4 py-3 text-left">Value</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60">
            {settingsLoading && settings.length === 0 ? (
              <tr>
                <td colSpan={3} className="px-4 py-4 text-center text-slate-400">
                  Loading settings…
                </td>
              </tr>
            ) : settings.length === 0 ? (
              <tr>
                <td colSpan={3} className="px-4 py-4 text-center text-slate-400">
                  No settings available.
                </td>
              </tr>
            ) : (
              settings.map((setting) => (
                <tr key={setting.name}>
                  <td className="px-4 py-3 font-medium text-slate-100">{setting.name}</td>
                  <td className="px-4 py-3 text-slate-200">{displayValue(setting.value)}</td>
                  <td className="px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={() => handleSettingUpdate(setting)}
                      className="rounded-md border border-slate-700 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
                      disabled={!canModerate || !server?.connected}
                    >
                      Update
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );

  const renderAuditTab = () => (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-cyan">Audit history</h2>
          <p className="text-xs text-slate-400">Recent actions proxied through the API.</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => fetchAuditLogs()}
            className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
          >
            Refresh
          </button>
          <button
            type="button"
            onClick={() => {
              void handleAuditExport();
            }}
            disabled={auditExporting}
            className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky disabled:cursor-not-allowed disabled:opacity-50"
          >
            {auditExporting ? "Exporting…" : "Download CSV"}
          </button>
        </div>
      </div>
      {auditError ? (
        <div className="rounded-lg border border-rose-900/50 bg-rose-950/40 px-4 py-3 text-sm text-rose-200">{auditError}</div>
      ) : null}
      {auditExportError ? (
        <div className="rounded-lg border border-amber-700/50 bg-amber-900/30 px-4 py-3 text-sm text-amber-200">
          {auditExportError}
        </div>
      ) : null}
      <div className="overflow-hidden rounded-xl border border-slate-800 bg-slate-950/60">
        <table className="min-w-full divide-y divide-slate-800 text-sm">
          <thead className="bg-slate-950/80 text-xs uppercase tracking-wide text-slate-400">
            <tr>
              <th className="px-4 py-3 text-left">Timestamp</th>
              <th className="px-4 py-3 text-left">Actor</th>
              <th className="px-4 py-3 text-left">Action</th>
              <th className="px-4 py-3 text-left">Result</th>
              <th className="px-4 py-3 text-left">Params hash</th>
              <th className="px-4 py-3 text-left">Error</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60">
            {auditLoading && auditLogs.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-4 text-center text-slate-400">
                  Loading audit log…
                </td>
              </tr>
            ) : auditLogs.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-4 text-center text-slate-400">
                  No audit entries recorded yet.
                </td>
              </tr>
            ) : (
              auditLogs.map((log) => (
                <tr key={log.id}>
                  <td className="px-4 py-3 text-slate-200">{new Date(log.timestamp).toLocaleString()}</td>
                  <td className="px-4 py-3 text-slate-200">{log.user_email ?? log.user_id ?? "system"}</td>
                  <td className="px-4 py-3 text-slate-100">{log.action}</td>
                  <td className="px-4 py-3 text-slate-200">{log.result_status}</td>
                  <td className="px-4 py-3 text-xs text-slate-400 font-mono">{log.params_sha256}</td>
                  <td className="px-4 py-3 text-slate-200">{log.error_message ?? ""}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );

  const renderOverview = () => (
    <div className="space-y-6">
      <section className="rounded-xl border border-slate-800 bg-slate-950/60 p-6 shadow-md shadow-slate-950/20">
        <h2 className="text-lg font-semibold text-cyan">Server summary</h2>
        <dl className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Name</dt>
            <dd className="text-sm text-slate-100">{server?.name}</dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Status</dt>
            <dd className="text-sm text-slate-100">{server?.connected ? "Online" : "Offline"}</dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Last seen</dt>
            <dd className="text-sm text-slate-100">{formatTimestamp(server?.connected_at ?? null)}</dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Created</dt>
            <dd className="text-sm text-slate-100">{server ? formatTimestamp(server.created_at) : ""}</dd>
          </div>
          {server?.description ? (
            <div className="sm:col-span-2">
              <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Description</dt>
              <dd className="text-sm text-slate-100">{server.description}</dd>
            </div>
          ) : null}
        </dl>
      </section>

      <section className="space-y-6">
        <div>
          <h2 className="text-lg font-semibold text-cyan">Critical actions</h2>
          <p className="text-xs text-slate-400">World save is available to moderators; stop is restricted to owners.</p>
        </div>
        <div className="grid gap-6 md:grid-cols-2">
          <RpcActionCard
            title="Save world"
            description="Flush world chunks to disk to prevent data loss."
            buttonLabel="Save world"
            disabled={!canModerate || !server?.connected}
            onSubmit={async () => handleServerSave()}
            fields={[]}
          />
          <RpcActionCard
            title="Stop server"
            description="Gracefully stop the server process."
            buttonLabel="Stop server"
            disabled={!canOwner || !server?.connected}
            onSubmit={async () => handleServerStop()}
            fields={[]}
          />
        </div>
      </section>

      {schema ? (
        <section className="rounded-xl border border-slate-800 bg-slate-950/60 p-6 shadow-inner shadow-slate-950/20">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold text-cyan">Discovered schema</h2>
              <p className="text-xs text-slate-400">Latest rpc.discover payload cached from the connected agent.</p>
            </div>
            <button
              type="button"
              onClick={() => loadServer()}
              className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
            >
              Refresh schema
            </button>
          </div>
          <pre className="mt-4 max-h-72 overflow-y-auto rounded bg-slate-900/70 px-4 py-3 text-xs text-slate-200">
            {JSON.stringify(schema, null, 2)}
          </pre>
        </section>
      ) : null}
    </div>
  );

  const renderActiveTab = () => {
    switch (activeTab) {
      case "players":
        return renderPlayersTab();
      case "gamerules":
        return renderGameRulesTab();
      case "settings":
        return renderSettingsTab();
      case "audit":
        return renderAuditTab();
      case "overview":
      default:
        return renderOverview();
    }
  };

  if (loading) {
    return (
      <div className="rounded-xl border border-slate-900 bg-slate-950/60 p-10 text-center text-slate-300">
        Loading server…
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-xl border border-rose-900/50 bg-rose-950/40 p-6 text-sm text-rose-200">{error}</div>
    );
  }

  if (!server) {
    return (
      <div className="rounded-xl border border-slate-900 bg-slate-950/50 p-10 text-center text-slate-300">Server not found.</div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2 text-xs uppercase tracking-wide text-slate-500">
            <Link to="/servers" className="text-sky hover:text-cyan">
              Servers
            </Link>
            <span>/</span>
            <span>{server.name}</span>
          </div>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-100">{server.name}</h1>
          <p className="text-sm text-slate-400">Manage allowlist, operators, settings, and audit history.</p>
        </div>
        <div className="flex flex-wrap items-center gap-3 text-sm">
          {statusBadge}
          <button
            type="button"
            onClick={() => loadServer()}
            className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:border-sky"
          >
            Refresh
          </button>
        </div>
      </div>

      <div className="overflow-hidden rounded-xl border border-slate-800 bg-slate-950/40">
        <nav className="flex flex-wrap gap-1 border-b border-slate-800 bg-slate-950/60 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-slate-400">
          {tabs.map((tab) => {
            const isActive = tab.id === activeTab;
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                className={`rounded-md px-3 py-1.5 transition ${isActive ? "bg-cyan text-slate-900" : "text-slate-300 hover:text-cyan"}`}
              >
                {tab.label}
              </button>
            );
          })}
        </nav>
      </div>

      <div className="grid gap-8 xl:grid-cols-[1.7fr_1fr]">
        <div>{renderActiveTab()}</div>
        <aside className="space-y-4">
          <div>
            <h2 className="text-lg font-semibold text-cyan">Live events</h2>
            <p className="text-xs text-slate-400">Minecraft notifications streamed from the agent.</p>
          </div>
          <EventStream events={events} />
        </aside>
      </div>
    </div>
  );
};

export default ServerDetailPage;

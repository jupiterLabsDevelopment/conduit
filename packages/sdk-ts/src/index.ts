import WS from "isomorphic-ws";

export type Role = "owner" | "moderator" | "viewer";

export interface AuthUser {
  id: string;
  email: string;
  role: Role;
}

export interface LoginResponse {
  token: string;
  user: AuthUser;
}

export interface AuditLogEntry {
  id: number;
  timestamp: string;
  user_id?: string;
  user_email?: string;
  action: string;
  params_sha256: string;
  result_status: string;
  error_message?: string;
}

export interface AuditExportOptions {
  from?: string | Date;
  to?: string | Date;
  limit?: number;
  signal?: AbortSignal;
}

export interface GameRulePreset {
  key: string;
  label: string;
  description: string;
  game_rules?: Record<string, unknown>;
  settings?: Record<string, unknown>;
}

export interface PresetApplicationResult {
  type: "gamerule" | "setting";
  name: string;
  value: unknown;
  status: "ok" | "error";
  message?: string;
}

export interface ApplyPresetResponse {
  preset: GameRulePreset;
  results: PresetApplicationResult[];
  duration_ms: number;
}

export interface ApiKeySummary {
  id: string;
  name: string;
  created_at: string;
}

export interface ApiKeyWithSecret extends ApiKeySummary {
  secret: string;
}

export interface ServerListItem {
  id: string;
  name: string;
  description?: string | null;
  connected: boolean;
  connected_at?: string | null;
  created_at: string;
}

export interface ServerDetail extends ServerListItem {}

export interface ConduitClientOptions {
  apiBase?: string;
  wsBase?: string;
  token?: string | null;
  fetchImpl?: typeof fetch;
  webSocketImpl?: WebSocketConstructor;
}

export interface JsonRpcRequest {
  jsonrpc?: string;
  id?: string | number | null;
  method: string;
  params?: unknown;
}

interface WebSocketConstructor {
  new (url: string, protocols?: string | string[]): WebSocketLike;
}

export interface WebSocketLike {
  readyState: number;
  onopen: ((event: unknown) => void) | null;
  onclose: ((event: unknown) => void) | null;
  onerror: ((event: unknown) => void) | null;
  onmessage: ((event: { data: any }) => void) | null;
  close(code?: number, reason?: string): void;
  send(data: any): void;
  addEventListener?: (type: string, listener: (...args: any[]) => void) => void;
  removeEventListener?: (type: string, listener: (...args: any[]) => void) => void;
}

const DEFAULT_HTTP_BASE = "http://localhost:8080";

const normalizeBase = (input?: string | null): string => {
  const trimmed = input?.trim();
  if (!trimmed) {
    return DEFAULT_HTTP_BASE;
  }
  return trimmed;
};

const deriveWsBase = (httpBase?: string | null): string => {
  const base = normalizeBase(httpBase);
  if (base.startsWith("http://")) {
    return `ws://${base.slice("http://".length)}`;
  }
  if (base.startsWith("https://")) {
    return `wss://${base.slice("https://".length)}`;
  }
  return base;
};

const resolveFetch = (custom?: typeof fetch): typeof fetch => {
  if (custom) {
    return custom;
  }
  if (typeof fetch !== "undefined") {
    return fetch;
  }
  throw new Error("No fetch implementation available. Provide one via options.fetchImpl");
};

const resolveWebSocket = (custom?: WebSocketConstructor): WebSocketConstructor => {
  if (custom) {
    return custom;
  }
  const globalWS = (globalThis as unknown as { WebSocket?: WebSocketConstructor }).WebSocket;
  if (globalWS) {
    return globalWS;
  }
  return WS as unknown as WebSocketConstructor;
};

export class ConduitClient {
  readonly apiBase: string;
  readonly wsBase: string;
  private token: string | null;
  private readonly fetchImpl: typeof fetch;
  private readonly WebSocketImpl: WebSocketConstructor;

  constructor(options: ConduitClientOptions = {}) {
    const apiBase = normalizeBase(options.apiBase).replace(/\/$/, "");
    this.apiBase = apiBase;
    this.wsBase = (options.wsBase ?? deriveWsBase(apiBase)).replace(/\/$/, "");
    this.token = options.token ?? null;
    this.fetchImpl = resolveFetch(options.fetchImpl);
    this.WebSocketImpl = resolveWebSocket(options.webSocketImpl);
  }

  setToken(token: string | null): void {
    this.token = token ?? null;
  }

  getToken(): string | null {
    return this.token;
  }

  async login(email: string, password: string): Promise<LoginResponse> {
    const res = await this.fetchJson<LoginResponse>("/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password })
    });
    this.setToken(res.token);
    return res;
  }

  async logout(): Promise<void> {
    await this.fetchJson<void>("/v1/auth/logout", {
      method: "POST"
    });
    this.setToken(null);
  }

  async bootstrap(email: string, password: string): Promise<void> {
    await this.fetchJson<void>("/v1/users/bootstrap", {
      method: "POST",
      body: JSON.stringify({ email, password })
    });
  }

  async listServers(): Promise<ServerListItem[]> {
    return this.fetchJson<ServerListItem[]>("/v1/servers");
  }

  async createServer(input: { name: string; description?: string | null }): Promise<{ id: string; agent_token: string }> {
    return this.fetchJson<{ id: string; agent_token: string }>("/v1/servers", {
      method: "POST",
      body: JSON.stringify(input)
    });
  }

  async getServer(id: string): Promise<ServerDetail> {
    return this.fetchJson<ServerDetail>(`/v1/servers/${id}`);
  }

  async getServerSchema(id: string): Promise<unknown> {
    return this.fetchJson<unknown>(`/v1/servers/${id}/schema`);
  }

  async listAuditLogs(id: string, limit?: number): Promise<AuditLogEntry[]> {
    const params = new URLSearchParams();
    if (limit != null) {
      params.set("limit", String(limit));
    }
    const suffix = params.size > 0 ? `?${params.toString()}` : "";
    return this.fetchJson<AuditLogEntry[]>(`/v1/servers/${id}/audit${suffix}`);
  }

  async listGameRulePresets(): Promise<GameRulePreset[]> {
    return this.fetchJson<GameRulePreset[]>("/v1/game-rule-presets");
  }

  async applyGameRulePreset(id: string, presetKey: string): Promise<ApplyPresetResponse> {
    return this.fetchJson<ApplyPresetResponse>(`/v1/servers/${id}/gamerules/apply-preset`, {
      method: "POST",
      body: JSON.stringify({ preset: presetKey })
    });
  }

  async exportAuditLogs(id: string, options?: AuditExportOptions): Promise<string> {
    const params = new URLSearchParams();
    const normalize = (value: string | Date): string => (value instanceof Date ? value.toISOString() : value);

    if (options?.from) {
      params.set("from", normalize(options.from));
    }
    if (options?.to) {
      params.set("to", normalize(options.to));
    }
    if (options?.limit != null) {
      params.set("limit", String(options.limit));
    }

    const suffix = params.size > 0 ? `?${params.toString()}` : "";
    const response = await this.request(`/v1/servers/${id}/audit/export${suffix}`, {
      method: "GET",
      headers: { Accept: "text/csv" },
      signal: options?.signal
    });

    const text = await response.text();
    if (!response.ok) {
      this.throwForError(response, text);
    }

    return text;
  }

  async callServerRpc<T = unknown>(id: string, method: string, params: unknown): Promise<T> {
    const result = await this.fetchJson<{ result: T } | T>(`/v1/servers/${id}/rpc`, {
      method: "POST",
      body: JSON.stringify({ method, params })
    });

    if (result && typeof result === "object" && "result" in result) {
      return (result as { result: T }).result;
    }
    return result as T;
  }

  openServerEvents(serverId: string): WebSocketLike {
    if (!this.token) {
      throw new Error("Authentication required to open event stream");
    }
    const socket = new this.WebSocketImpl(`${this.wsBase}/ws/servers/${serverId}/events`, ["jwt", this.token]);
    return socket;
  }

  async listApiKeys(): Promise<ApiKeySummary[]> {
    return this.fetchJson<ApiKeySummary[]>("/v1/api-keys");
  }

  async createApiKey(name: string): Promise<ApiKeyWithSecret> {
    return this.fetchJson<ApiKeyWithSecret>("/v1/api-keys", {
      method: "POST",
      body: JSON.stringify({ name })
    });
  }

  async deleteApiKey(id: string): Promise<void> {
    await this.fetchJson<void>(`/v1/api-keys/${id}`, {
      method: "DELETE"
    });
  }

  private async request(path: string, init: RequestInit = {}): Promise<Response> {
    const headers = new Headers(init.headers ?? {});
    if (!headers.has("Content-Type") && init.body) {
      headers.set("Content-Type", "application/json");
    }
    if (this.token) {
      headers.set("Authorization", `Bearer ${this.token}`);
    }

    const fetchImpl = this.fetchImpl;
    return fetchImpl(`${this.apiBase}${path}`, {
      ...init,
      headers
    });
  }

  private throwForError(response: Response, text: string): never {
    let message = response.statusText;
    const trimmed = text.trim();
    if (trimmed) {
      try {
        const parsed = JSON.parse(trimmed) as { error?: unknown };
        if (parsed && typeof parsed.error === "string" && parsed.error.trim() !== "") {
          message = parsed.error;
        } else if (!message) {
          message = trimmed;
        }
      } catch {
        if (!message) {
          message = trimmed;
        }
      }
    }

    if (!message) {
      message = `Request failed (${response.status})`;
    }
    throw new Error(message);
  }

  private async fetchJson<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await this.request(path, init);

    if (response.status === 204) {
      return undefined as T;
    }

    const text = await response.text();
    if (!response.ok) {
      this.throwForError(response, text);
    }

    if (!text) {
      return undefined as T;
    }

    try {
      return JSON.parse(text) as T;
    } catch (err) {
      throw new Error(`Failed to parse response: ${String(err)}`);
    }
  }
}

export function createClient(options?: ConduitClientOptions): ConduitClient {
  return new ConduitClient(options);
}

export default ConduitClient;

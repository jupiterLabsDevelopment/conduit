import {
  ConduitClient,
  type ApiKeySummary,
  type ApiKeyWithSecret,
  type AuditExportOptions,
  type AuditLogEntry,
  type ApplyPresetResponse,
  type AuthUser,
  type GameRulePreset,
  type LoginResponse,
  type PresetApplicationResult,
  type Role,
  type ServerDetail,
  type ServerListItem,
} from "@conduit/sdk";

const DEFAULT_HTTP_BASE = "http://localhost:8080";

const normalizeHttpBase = (input?: string | null): string => {
  const trimmed = input?.trim();
  if (!trimmed) {
    return DEFAULT_HTTP_BASE;
  }
  return trimmed;
};

function deriveWsBase(httpBase?: string | null): string {
  const base = normalizeHttpBase(httpBase);
  if (base.startsWith("http://")) {
    return `ws://${base.slice("http://".length)}`;
  }
  if (base.startsWith("https://")) {
    return `wss://${base.slice("https://".length)}`;
  }
  return base;
}

const API_BASE = normalizeHttpBase(import.meta.env.VITE_API_BASE as string | undefined);
const explicitWsBase = (import.meta.env.VITE_API_WS as string | undefined)?.trim();
const API_WS_BASE = explicitWsBase && explicitWsBase.length > 0 ? explicitWsBase : deriveWsBase(API_BASE);

export type {
  ApiKeySummary,
  ApiKeyWithSecret,
  AuditExportOptions,
  AuditLogEntry,
  ApplyPresetResponse,
  AuthUser,
  GameRulePreset,
  LoginResponse,
  PresetApplicationResult,
  Role,
  ServerDetail,
  ServerListItem,
};
export type ApiClient = ConduitClient;

export const apiClient = new ConduitClient({ apiBase: API_BASE, wsBase: API_WS_BASE });

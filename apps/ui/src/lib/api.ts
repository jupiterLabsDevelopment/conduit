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

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? "http://localhost:8080";
const API_WS_BASE = (import.meta.env.VITE_API_WS as string | undefined) ?? deriveWsBase(API_BASE);

function deriveWsBase(httpBase: string): string {
  if (httpBase.startsWith("http://")) {
    return `ws://${httpBase.slice("http://".length)}`;
  }
  if (httpBase.startsWith("https://")) {
    return `wss://${httpBase.slice("https://".length)}`;
  }
  return httpBase;
}

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

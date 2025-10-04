import { useEffect, useState } from "react";

import { useAuth } from "../context/AuthContext";

export interface ServerEventFrame {
  method: string;
  params?: unknown;
  ts: number;
  raw: Record<string, unknown>;
}

export const useServerEvents = (serverId: string | undefined) => {
  const { api, token } = useAuth();
  const [events, setEvents] = useState<ServerEventFrame[]>([]);

  useEffect(() => {
    if (!serverId || !token) {
      return;
    }

    const socket = api.openServerEvents(serverId);
    socket.onmessage = (evt) => {
      try {
        const frame = JSON.parse(evt.data) as Record<string, unknown>;
        const method = typeof frame.method === "string" ? frame.method : "unknown";
        if (!method.startsWith("minecraft:notification/")) {
          return;
        }
  setEvents((prev: ServerEventFrame[]) => {
          const next = [
            {
              method,
              params: frame.params,
              raw: frame,
              ts: Date.now(),
            },
            ...prev,
          ];
          return next.slice(0, 50);
        });
      } catch (err) {
        console.warn("Failed to parse event", err);
      }
    };
    socket.onerror = (evt) => {
      console.warn("Event socket error", evt);
    };

    return () => {
      socket.close();
    };
  }, [api, serverId, token]);

  return events;
};

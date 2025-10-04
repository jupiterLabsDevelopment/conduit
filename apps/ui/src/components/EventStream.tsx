import { useMemo } from "react";

import type { ServerEventFrame } from "../hooks/useServerEvents";

interface Props {
  events: ServerEventFrame[];
}

const formatMethod = (method: string) => method.replace("minecraft:notification/", "");

export const EventStream = ({ events }: Props) => {
  const grouped = useMemo(() => events, [events]);

  if (grouped.length === 0) {
    return (
      <div className="rounded-lg border border-slate-800 bg-slate-950/50 p-4 text-sm text-slate-400">
        Waiting for notifications…
      </div>
    );
  }

  return (
    <div className="space-y-3">
  {grouped.map((event: ServerEventFrame) => (
        <div
          key={`${event.method}-${event.ts}`}
          className="rounded-lg border border-slate-800 bg-slate-950/70 p-4 shadow-sm shadow-slate-950/10"
        >
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span className="font-semibold uppercase tracking-wide text-cyan">{formatMethod(event.method)}</span>
            <span>{new Date(event.ts).toLocaleTimeString()}</span>
          </div>
          <pre className="mt-3 overflow-x-auto rounded bg-slate-900/80 px-3 py-2 text-xs text-slate-200">
            {JSON.stringify(event.raw, null, 2)}
          </pre>
        </div>
      ))}
    </div>
  );
};

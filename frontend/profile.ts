// The shape the unit's operations answer with, as the overlays record it.

export const REQUESTS_OBJECT = "ext_profiler_requests";
export const REQUESTS_KEY = ["ext", "profiler", "requests"] as const;
export const LIST_HASH = "#/ext/profiler";

export type ProfileSummary = {
  token: string;
  method: string;
  path: string;
  route?: string;
  operation?: string;
  status: number;
  started_at: string;
  duration_ms: number;
  queries: number;
  sql_ms: number;
  writes: number;
  ai_calls: number;
  tool_calls: number;
  redis: number;
  http: number;
};

export type Query = {
  sql: string;
  /** Each argument as the statement received it, `$1` first. */
  args?: string[];
  /** The product function that ran it, with its file and line. */
  caller?: string;
  at_ms: number;
  duration_ms: number;
  rows: number;
  error?: string;
};

export type Write = {
  entity: string;
  action: string;
  id: string;
  at_ms: number;
};

export type HookRun = {
  unit: string;
  kind: string;
  name: string;
  at_ms: number;
  duration_ms: number;
  error?: string;
};

export type LogLine = {
  level: string;
  message: string;
  attrs?: Record<string, string>;
  at_ms: number;
};

/** A call out of the request that is not SQL. */
export type SpanKind =
  | "stage"
  | "controller"
  | "ai"
  | "tool"
  | "redis"
  | "http"
  | "event";

export type Span = {
  kind: SpanKind;
  name: string;
  at_ms: number;
  duration_ms: number;
  detail?: Record<string, string>;
  caller?: string;
  error?: string;
};

/** One side of the exchange, as the profiler kept it. */
export type Message = {
  headers: Record<string, string[]>;
  body?: string;
  body_bytes: number;
  cut?: boolean;
  binary?: boolean;
  streamed?: boolean;
};

export type Profile = {
  token: string;
  method: string;
  path: string;
  route?: string;
  operation?: string;
  status: number;
  started_at: string;
  duration_ms: number;
  actor_type?: string;
  actor_id?: string;
  queries: Query[];
  writes: Write[];
  hooks: HookRun[];
  logs: LogLine[];
  spans: Span[];
  request?: Message;
  response?: Message;
  dropped: number;
};

/**
 * How many times each statement ran. A statement run more than once in one
 * request is the N+1 shape the SQL panel flags.
 */
export function repeatedStatements(
  queries: readonly Query[],
): Map<string, number> {
  const counts = new Map<string, number>();
  for (const q of queries) counts.set(q.sql, (counts.get(q.sql) ?? 0) + 1);
  return counts;
}

/** One row of the timeline: anything the request did, on one time axis. */
export type TimelineEntry = {
  key: string;
  kind: SpanKind | "sql" | "hook";
  name: string;
  at_ms: number;
  duration_ms: number;
  caller?: string;
  error?: string;
};

/** Every timed thing the request did, in the order it started. */
export function timeline(p: Profile): TimelineEntry[] {
  const entries: TimelineEntry[] = [
    ...p.queries.map((q, i) => ({
      key: `sql${i}`,
      kind: "sql" as const,
      name: q.sql,
      at_ms: q.at_ms,
      duration_ms: q.duration_ms,
      caller: q.caller,
      error: q.error,
    })),
    ...p.hooks.map((h, i) => ({
      key: `hook${i}`,
      kind: "hook" as const,
      name: `${h.unit} · ${h.name}`,
      at_ms: h.at_ms,
      duration_ms: h.duration_ms,
      error: h.error,
    })),
    ...p.spans.map((s, i) => ({
      key: `span${i}`,
      kind: s.kind,
      name: s.name,
      at_ms: s.at_ms,
      duration_ms: s.duration_ms,
      caller: s.caller,
      error: s.error,
    })),
  ];
  return entries.sort((a, b) => a.at_ms - b.at_ms);
}

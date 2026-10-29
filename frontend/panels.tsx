import { useLocale, useT } from "@margince/frontend/app";
import { Badge, DataTable, EmptyState } from "@margince/frontend/design-system";
import { CopyButton, runnableScript, runnableSql } from "./copy";
import { count, detailLine, ms } from "./format";
import {
  type HookRun,
  type LogLine,
  type Profile,
  type Query,
  repeatedStatements,
  type Span,
  type TimelineEntry,
  timeline,
  type Write,
} from "./profile";
import { ordered, useView, withAt } from "./view";

// One table per kind of thing a request did. Every time is milliseconds from
// the start of the request.

const KIND_TONE = {
  stage: "default",
  controller: "accent",
  sql: "info",
  hook: "default",
  ai: "ai",
  tool: "default",
  redis: "warning",
  http: "discovery",
  event: "success",
} as const;

/**
 * Where an entry sits within the request, as a bar the width of its share of
 * the whole. A unit ships no stylesheet, so the bar is drawn inline in the
 * text colour.
 */
function Bar({
  entry,
  total,
}: Readonly<{ entry: TimelineEntry; total: number }>) {
  const left = total > 0 ? (entry.at_ms / total) * 100 : 0;
  const width =
    total > 0 ? Math.max((entry.duration_ms / total) * 100, 0.5) : 0;
  return (
    <div style={{ position: "relative", height: 8, minWidth: 160 }}>
      <div
        style={{
          position: "absolute",
          left: `${Math.min(left, 99.5)}%`,
          width: `${Math.min(width, 100 - left)}%`,
          height: "100%",
          borderRadius: 2,
          background: "currentColor",
          opacity: 0.45,
        }}
      />
    </div>
  );
}

/** The product function behind an entry, and where it is. */
function Caller({ caller }: Readonly<{ caller?: string }>) {
  if (!caller) return null;
  const [fn, at] = caller.split(" ");
  return (
    <div className="t-caption">
      <code title={at}>{fn}</code>
    </div>
  );
}

export function TimelinePanel({ profile }: Readonly<{ profile: Profile }>) {
  const t = useT();
  const { locale } = useLocale();
  const { order, showAt } = useView();
  const entries = timeline(profile);
  if (entries.length === 0)
    return <EmptyState>{t("extProfiler.none")}</EmptyState>;
  return (
    <DataTable<TimelineEntry>
      label={t("extProfiler.panel.timeline")}
      rows={ordered(entries, order)}
      rowKey={(row) => row.key}
      columns={withAt(
        [
          {
            key: "at",
            header: t("extProfiler.col.at"),
            align: "end",
            render: (row) => ms(row.at_ms, locale),
          },
          {
            key: "duration",
            header: t("extProfiler.col.duration"),
            align: "end",
            render: (row) => ms(row.duration_ms, locale),
          },
          {
            key: "kind",
            header: t("extProfiler.col.kind"),
            render: (row) => (
              <Badge tone={row.error ? "danger" : KIND_TONE[row.kind]}>
                {t(`extProfiler.kind.${row.kind}`)}
              </Badge>
            ),
          },
          {
            key: "bar",
            header: t("extProfiler.panel.timeline"),
            grow: true,
            render: (row) => <Bar entry={row} total={profile.duration_ms} />,
          },
          {
            key: "caller",
            header: t("extProfiler.col.function"),
            render: (row) => <Caller caller={row.caller} />,
          },
          {
            key: "name",
            header: t("extProfiler.col.name"),
            render: (row) => (
              <code
                title={row.name}
                style={{
                  display: "inline-block",
                  maxWidth: "32rem",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {row.name}
              </code>
            ),
          },
        ],
        showAt,
      )}
    />
  );
}

export function SpanPanel({
  label,
  spans,
}: Readonly<{ label: string; spans: Span[] }>) {
  const t = useT();
  const { locale } = useLocale();
  const { order, showAt } = useView();
  if (spans.length === 0)
    return <EmptyState>{t("extProfiler.none")}</EmptyState>;
  return (
    <DataTable<Span & { n: number }>
      label={label}
      rows={ordered(
        spans.map((s, i) => ({ ...s, n: i })),
        order,
      )}
      rowKey={(row) => String(row.n)}
      columns={withAt(
        [
          {
            key: "at",
            header: t("extProfiler.col.at"),
            align: "end",
            render: (row) => ms(row.at_ms, locale),
          },
          {
            key: "duration",
            header: t("extProfiler.col.duration"),
            align: "end",
            render: (row) => ms(row.duration_ms, locale),
          },
          {
            key: "name",
            header: t("extProfiler.col.name"),
            render: (row) => <code>{row.name}</code>,
          },
          {
            key: "caller",
            header: t("extProfiler.col.function"),
            render: (row) => <Caller caller={row.caller} />,
          },
          {
            key: "detail",
            header: t("extProfiler.col.detail"),
            grow: true,
            render: (row) => (
              <>
                {row.error ? <Badge tone="danger">{row.error}</Badge> : null}{" "}
                <code style={{ whiteSpace: "pre-wrap" }}>
                  {detailLine(row.detail)}
                </code>
              </>
            ),
          },
        ],
        showAt,
      )}
    />
  );
}

export function QueryPanel({ queries }: Readonly<{ queries: Query[] }>) {
  const t = useT();
  const { locale } = useLocale();
  const { order, showAt } = useView();
  const repeated = repeatedStatements(queries);
  if (queries.length === 0)
    return <EmptyState>{t("extProfiler.none")}</EmptyState>;
  return (
    <>
      <p>
        <CopyButton
          label={t("extProfiler.copy.allSql")}
          text={() => runnableScript(queries)}
        />
      </p>
      <DataTable<Query & { n: number }>
        label={t("extProfiler.panel.sql")}
        rows={ordered(
          queries.map((q, i) => ({ ...q, n: i + 1 })),
          order,
        )}
        rowKey={(row) => String(row.n)}
        columns={withAt(
          [
            {
              key: "n",
              header: "#",
              align: "end",
              render: (row) => String(row.n),
            },
            {
              key: "at",
              header: t("extProfiler.col.at"),
              align: "end",
              render: (row) => ms(row.at_ms, locale),
            },
            {
              key: "duration",
              header: t("extProfiler.col.duration"),
              align: "end",
              render: (row) => ms(row.duration_ms, locale),
            },
            {
              key: "rows",
              header: t("extProfiler.col.rows"),
              align: "end",
              render: (row) => count(row.rows, locale),
            },
            {
              key: "sql",
              header: t("extProfiler.col.statement"),
              grow: true,
              render: (row) => {
                const times = repeated.get(row.sql) ?? 0;
                return (
                  <>
                    {row.error ? (
                      <Badge tone="danger">{row.error}</Badge>
                    ) : null}
                    {times > 1 ? (
                      <Badge tone="warning">
                        {t("extProfiler.repeated", { count: String(times) })}
                      </Badge>
                    ) : null}
                    <code style={{ whiteSpace: "pre-wrap" }}>{row.sql}</code>
                    <Caller caller={row.caller} />
                    {row.args && row.args.length > 0 ? (
                      <div className="t-caption">
                        <code style={{ whiteSpace: "pre-wrap" }}>
                          {row.args
                            .map((a, i) => `$${i + 1} = ${a}`)
                            .join("  ")}
                        </code>
                      </div>
                    ) : null}
                    <div>
                      <CopyButton
                        label={t("extProfiler.copy.sql")}
                        text={() => runnableSql(row)}
                      />
                    </div>
                  </>
                );
              },
            },
          ],
          showAt,
        )}
      />
    </>
  );
}

export function WritePanel({ writes }: Readonly<{ writes: Write[] }>) {
  const t = useT();
  const { locale } = useLocale();
  const { order, showAt } = useView();
  if (writes.length === 0)
    return <EmptyState>{t("extProfiler.none")}</EmptyState>;
  return (
    <DataTable<Write>
      label={t("extProfiler.panel.writes")}
      rows={ordered(writes, order)}
      rowKey={(row) => `${row.entity}:${row.id}:${row.at_ms}`}
      columns={withAt(
        [
          {
            key: "at",
            header: t("extProfiler.col.at"),
            align: "end",
            render: (row) => ms(row.at_ms, locale),
          },
          {
            key: "action",
            header: t("extProfiler.col.action"),
            render: (row) => `${row.entity}.${row.action}`,
          },
          {
            key: "id",
            header: "ID",
            grow: true,
            render: (row) => <code>{row.id}</code>,
          },
        ],
        showAt,
      )}
    />
  );
}

export function HookPanel({ hooks }: Readonly<{ hooks: HookRun[] }>) {
  const t = useT();
  const { locale } = useLocale();
  const { order, showAt } = useView();
  if (hooks.length === 0)
    return <EmptyState>{t("extProfiler.none")}</EmptyState>;
  return (
    <DataTable<HookRun>
      label={t("extProfiler.panel.hooks")}
      rows={ordered(hooks, order)}
      rowKey={(row) => `${row.unit}:${row.name}:${row.at_ms}`}
      columns={withAt(
        [
          {
            key: "at",
            header: t("extProfiler.col.at"),
            align: "end",
            render: (row) => ms(row.at_ms, locale),
          },
          {
            key: "duration",
            header: t("extProfiler.col.duration"),
            align: "end",
            render: (row) => ms(row.duration_ms, locale),
          },
          {
            key: "hook",
            header: t("extProfiler.col.hook"),
            grow: true,
            render: (row) => `${row.unit} · ${row.kind} · ${row.name}`,
          },
          {
            key: "error",
            header: t("extProfiler.col.outcome"),
            render: (row) =>
              row.error ? <Badge tone="danger">{row.error}</Badge> : "",
          },
        ],
        showAt,
      )}
    />
  );
}

function levelTone(level: string) {
  if (level === "ERROR") return "danger";
  if (level === "WARN") return "warning";
  return "default";
}

export function LogPanel({ logs }: Readonly<{ logs: LogLine[] }>) {
  const t = useT();
  const { locale } = useLocale();
  const { order, showAt } = useView();
  if (logs.length === 0)
    return <EmptyState>{t("extProfiler.none")}</EmptyState>;
  return (
    <DataTable<LogLine>
      label={t("extProfiler.panel.logs")}
      rows={ordered(logs, order)}
      rowKey={(row) => `${row.at_ms}:${row.message}`}
      columns={withAt(
        [
          {
            key: "at",
            header: t("extProfiler.col.at"),
            align: "end",
            render: (row) => ms(row.at_ms, locale),
          },
          {
            key: "level",
            header: t("extProfiler.col.level"),
            render: (row) => (
              <Badge tone={levelTone(row.level)}>{row.level}</Badge>
            ),
          },
          {
            key: "message",
            header: t("extProfiler.col.message"),
            grow: true,
            render: (row) => (
              <>
                {row.message} <code>{detailLine(row.attrs)}</code>
              </>
            ),
          },
        ],
        showAt,
      )}
    />
  );
}

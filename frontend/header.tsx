import { formatDateTime, useLocale, useT } from "@margince/frontend/app";
import { Badge, Button } from "@margince/frontend/design-system";
import { CopyButton, curlCommand, runnableScript } from "./copy";
import { count, MISSING, ms, statusTone } from "./format";
import { LIST_HASH, type Profile, type SpanKind } from "./profile";

// The top of one request's page: a link back to the list, the request as the
// page title, and one line each for where it went, what it cost and who made it.

function readerZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

export function BackToRequests() {
  const t = useT();
  return (
    <Button
      variant="link"
      onClick={() => {
        window.location.hash = LIST_HASH;
      }}
    >
      {`← ${t("extProfiler.title")}`}
    </Button>
  );
}

export function RequestHeader({ profile: p }: Readonly<{ profile: Profile }>) {
  const t = useT();
  const { locale } = useLocale();
  const spans = (kind: SpanKind) => p.spans.filter((s) => s.kind === kind);
  const sqlMs = p.queries.reduce((sum, q) => sum + q.duration_ms, 0);
  const totals: [string, string][] = [
    [
      t("extProfiler.panel.sql"),
      `${count(p.queries.length, locale)} · ${ms(sqlMs, locale)}`,
    ],
    [t("extProfiler.panel.ai"), count(spans("ai").length, locale)],
    [t("extProfiler.panel.tool"), count(spans("tool").length, locale)],
    [t("extProfiler.panel.redis"), count(spans("redis").length, locale)],
    [t("extProfiler.panel.http"), count(spans("http").length, locale)],
    [t("extProfiler.panel.event"), count(spans("event").length, locale)],
    [t("extProfiler.panel.writes"), count(p.writes.length, locale)],
    [t("extProfiler.panel.logs"), count(p.logs.length, locale)],
  ];
  return (
    <>
      <div>
        <BackToRequests />
        {" / "}
        <h1
          style={{
            display: "inline",
            fontSize: "inherit",
            fontWeight: 600,
            wordBreak: "break-all",
          }}
        >
          <code>{`${p.method} ${p.path}`}</code>
        </h1>
      </div>
      <p>
        <Badge tone={statusTone(p.status)}>{String(p.status)}</Badge>{" "}
        <strong>{ms(p.duration_ms, locale)}</strong>
        {" · "}
        {p.operation ?? MISSING}
        {p.route ? (
          <>
            {" · "}
            <code>{p.route}</code>
          </>
        ) : null}
        {" · "}
        {formatDateTime(p.started_at, locale, readerZone())}
      </p>
      <p>
        {totals.map(([label, value], i) => (
          <span key={label}>
            {i > 0 ? " · " : ""}
            {label} <strong>{value}</strong>
          </span>
        ))}
      </p>
      <p className="t-caption">
        {p.actor_type ?? MISSING} <code>{p.actor_id ?? MISSING}</code>
        {" · "}
        {t("extProfiler.token")} <code>{p.token}</code>
      </p>
      <p>
        <CopyButton
          label={t("extProfiler.copy.url")}
          text={() => window.location.origin + p.path}
        />
        {" · "}
        <CopyButton
          label={t("extProfiler.copy.curl")}
          text={() =>
            curlCommand(window.location.origin, p.method, p.path, p.request)
          }
        />
        {" · "}
        <CopyButton
          label={t("extProfiler.copy.allSql")}
          text={() => runnableScript(p.queries)}
        />
        {" · "}
        <CopyButton label={t("extProfiler.copy.token")} text={() => p.token} />
      </p>
      {p.dropped > 0 ? (
        <p>{t("extProfiler.dropped", { count: String(p.dropped) })}</p>
      ) : null}
    </>
  );
}

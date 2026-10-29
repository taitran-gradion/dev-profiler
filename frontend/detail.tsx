import { api, QueryStates, throwProblem } from "@margince/frontend/api";
import { useT } from "@margince/frontend/app";
import { SegmentedControl } from "@margince/frontend/design-system";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { ExchangePanel } from "./exchange";
import { BackToRequests, RequestHeader } from "./header";
import {
  HookPanel,
  LogPanel,
  QueryPanel,
  SpanPanel,
  TimelinePanel,
  WritePanel,
} from "./panels";
import type { Profile, SpanKind } from "./profile";
import { ViewControls } from "./view";

const PANELS = [
  "timeline",
  "exchange",
  "sql",
  "ai",
  "tool",
  "redis",
  "http",
  "event",
  "writes",
  "hooks",
  "logs",
] as const;
type PanelName = (typeof PANELS)[number];

// The span kinds with a tab of their own; middleware and the controller show
// only on the timeline.
type SpanPanelKind = Extract<
  SpanKind,
  "ai" | "tool" | "redis" | "http" | "event"
>;

const SPAN_PANELS: readonly SpanPanelKind[] = [
  "ai",
  "tool",
  "redis",
  "http",
  "event",
];

function isSpanPanel(panel: PanelName): panel is SpanPanelKind {
  return (SPAN_PANELS as readonly string[]).includes(panel);
}

export function ProfileDetail({ token }: Readonly<{ token: string }>) {
  const t = useT();
  const [panel, setPanel] = useState<PanelName>("timeline");
  const profile = useQuery({
    queryKey: ["ext", "profiler", "request", token],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/ext/profiler/request", {
        params: { query: { token } },
      });
      if (error || !response.ok) throwProblem(error);
      return data as Profile;
    },
  });
  const p = profile.data;
  const spans = (kind: SpanPanelKind) =>
    p ? p.spans.filter((s) => s.kind === kind) : [];
  return (
    <div className="wrap">
      <QueryStates query={profile} pendingLabel={t("extProfiler.title")}>
        {p ? (
          <>
            <RequestHeader profile={p} />
            <SegmentedControl<PanelName>
              label={t("extProfiler.col.request")}
              options={PANELS}
              value={panel}
              onChange={setPanel}
              labels={{
                timeline: t("extProfiler.panel.timeline"),
                exchange: t("extProfiler.panel.exchange"),
                sql: t("extProfiler.panel.sql"),
                ai: t("extProfiler.panel.ai"),
                tool: t("extProfiler.panel.tool"),
                redis: t("extProfiler.panel.redis"),
                http: t("extProfiler.panel.http"),
                event: t("extProfiler.panel.event"),
                writes: t("extProfiler.panel.writes"),
                hooks: t("extProfiler.panel.hooks"),
                logs: t("extProfiler.panel.logs"),
              }}
              counts={{
                sql: p.queries.length,
                ai: spans("ai").length,
                tool: spans("tool").length,
                redis: spans("redis").length,
                http: spans("http").length,
                event: spans("event").length,
                writes: p.writes.length,
                hooks: p.hooks.length,
                logs: p.logs.length,
              }}
            />
            <ViewControls>
              <PanelBody panel={panel} profile={p} />
            </ViewControls>
          </>
        ) : null}
      </QueryStates>
      {profile.isError ? <BackToRequests /> : null}
    </div>
  );
}

function PanelBody({
  panel,
  profile,
}: Readonly<{ panel: PanelName; profile: Profile }>) {
  const t = useT();
  if (isSpanPanel(panel)) {
    return (
      <SpanPanel
        label={t(`extProfiler.panel.${panel}`)}
        spans={profile.spans.filter((s) => s.kind === panel)}
      />
    );
  }
  switch (panel) {
    case "timeline":
      return <TimelinePanel profile={profile} />;
    case "exchange":
      return <ExchangePanel profile={profile} />;
    case "sql":
      return <QueryPanel queries={profile.queries} />;
    case "writes":
      return <WritePanel writes={profile.writes} />;
    case "hooks":
      return <HookPanel hooks={profile.hooks} />;
    case "logs":
      return <LogPanel logs={profile.logs} />;
  }
}

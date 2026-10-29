import { api, QueryStates, throwProblem } from "@margince/frontend/api";
import {
  formatDateTime,
  useCan,
  useLocale,
  useT,
} from "@margince/frontend/app";
import {
  Badge,
  Button,
  DataTable,
  EmptyState,
  SectionHeader,
  SegmentedControl,
} from "@margince/frontend/design-system";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, useSyncExternalStore } from "react";
import { ProfileDetail } from "./detail";
import { count, ms, statusTone } from "./format";
import {
  LIST_HASH,
  type ProfileSummary,
  REQUESTS_KEY,
  REQUESTS_OBJECT,
} from "./profile";

// #/ext/profiler: the caller's own recent requests. The list polls, so a
// request made in another tab shows up while this is open.
// #/ext/profiler/<token>: what one of them did, as its own page, so the
// browser's back returns to the list and a profile can be linked to.

const POLL_MS = 3000;

const LIST_ORDERS = ["newest", "slowest"] as const;
type ListOrder = (typeof LIST_ORDERS)[number];

/** The answer already comes newest first. */
function inOrder(rows: ProfileSummary[], order: ListOrder): ProfileSummary[] {
  if (order === "newest") return rows;
  return [...rows].sort((a, b) => b.duration_ms - a.duration_ms);
}

// The core router is not on the unit surface, so the screen reads its own
// segment of the address.
function subscribeToHash(onChange: () => void) {
  window.addEventListener("hashchange", onChange);
  return () => window.removeEventListener("hashchange", onChange);
}

function openToken(): string | undefined {
  const [path = ""] = window.location.hash.split("?");
  const token = path.slice(LIST_HASH.length + 1);
  return path.startsWith(`${LIST_HASH}/`) && token ? token : undefined;
}

function readerZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

export default function ProfilerScreen() {
  const token = useSyncExternalStore(subscribeToHash, openToken);
  return token ? <ProfileDetail token={token} /> : <RequestList />;
}

function RequestList() {
  const t = useT();
  const { locale } = useLocale();
  const queryClient = useQueryClient();
  const canClear = useCan(REQUESTS_OBJECT, "delete");
  const [order, setOrder] = useState<ListOrder>("newest");
  const requests = useQuery({
    queryKey: REQUESTS_KEY,
    refetchInterval: POLL_MS,
    queryFn: async () => {
      const { data, error, response } = await api.GET("/ext/profiler/requests");
      if (error || !response.ok) throwProblem(error);
      return (data?.requests ?? []) as ProfileSummary[];
    },
  });
  const clear = useMutation({
    mutationFn: async () => {
      const { error, response } = await api.POST("/ext/profiler/clear", {
        body: {},
      });
      if (error || !response.ok) throwProblem(error);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: REQUESTS_KEY }),
  });
  return (
    <div className="wrap">
      <SectionHeader title={t("extProfiler.title")} level={1} />
      <p>{t("extProfiler.sub")}</p>
      <div
        style={{
          display: "flex",
          gap: "1rem",
          alignItems: "center",
          flexWrap: "wrap",
        }}
      >
        <SegmentedControl<ListOrder>
          label={t("extProfiler.order")}
          options={LIST_ORDERS}
          value={order}
          onChange={setOrder}
          labels={{
            newest: t("extProfiler.order.newest"),
            slowest: t("extProfiler.order.slowest"),
          }}
        />
        <Button
          variant="ghost"
          pending={clear.isPending}
          reason={canClear ? undefined : t("extProfiler.clearNoGrant")}
          onClick={() => clear.mutate()}
        >
          {t("extProfiler.clear")}
        </Button>
      </div>
      <QueryStates query={requests} pendingLabel={t("extProfiler.title")}>
        {requests.data && requests.data.length > 0 ? (
          <DataTable<ProfileSummary>
            label={t("extProfiler.title")}
            rows={inOrder(requests.data, order)}
            rowKey={(row) => row.token}
            onRowClick={(row) => {
              window.location.hash = `${LIST_HASH}/${row.token}`;
            }}
            columns={[
              {
                key: "at",
                header: t("extProfiler.col.at"),
                render: (row) =>
                  formatDateTime(row.started_at, locale, readerZone()),
              },
              {
                key: "status",
                header: t("extProfiler.col.status"),
                render: (row) => (
                  <Badge tone={statusTone(row.status)}>
                    {String(row.status)}
                  </Badge>
                ),
              },
              {
                key: "request",
                header: t("extProfiler.col.request"),
                grow: true,
                render: (row) => (
                  <code style={{ wordBreak: "break-all" }}>
                    {row.method} {row.path}
                  </code>
                ),
              },
              {
                key: "duration",
                header: t("extProfiler.col.duration"),
                align: "end",
                render: (row) => ms(row.duration_ms, locale),
              },
              {
                key: "queries",
                header: t("extProfiler.col.queries"),
                align: "end",
                render: (row) =>
                  `${count(row.queries, locale)} · ${ms(row.sql_ms, locale)}`,
              },
              {
                key: "ai",
                header: t("extProfiler.panel.ai"),
                align: "end",
                render: (row) => count(row.ai_calls, locale),
              },
              {
                key: "tools",
                header: t("extProfiler.panel.tool"),
                align: "end",
                render: (row) => count(row.tool_calls, locale),
              },
              {
                key: "redis",
                header: t("extProfiler.panel.redis"),
                align: "end",
                render: (row) => count(row.redis, locale),
              },
              {
                key: "http",
                header: t("extProfiler.panel.http"),
                align: "end",
                render: (row) => count(row.http, locale),
              },
              {
                key: "writes",
                header: t("extProfiler.col.writes"),
                align: "end",
                render: (row) => count(row.writes, locale),
              },
            ]}
          />
        ) : (
          <EmptyState>{t("extProfiler.empty")}</EmptyState>
        )}
      </QueryStates>
    </div>
  );
}

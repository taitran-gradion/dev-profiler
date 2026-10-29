import { formatBytes, useLocale, useT } from "@margince/frontend/app";
import {
  DataTable,
  EmptyState,
  SectionHeader,
} from "@margince/frontend/design-system";
import { CopyButton } from "./copy";
import type { Message, Profile } from "./profile";

// What the client sent and what it got back: headers, then the body, with a
// JSON body laid out to read.

/** A JSON body indented; any other text as it came. */
export function readableBody(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return body;
  }
}

type Header = { name: string; value: string };

function headerRows(headers: Record<string, string[]>): Header[] {
  return Object.entries(headers)
    .flatMap(([name, values]) => values.map((value) => ({ name, value })))
    .sort((a, b) => a.name.localeCompare(b.name));
}

function MessageView({
  title,
  message,
}: Readonly<{ title: string; message?: Message }>) {
  const t = useT();
  const { locale } = useLocale();
  if (!message) {
    return (
      <>
        <SectionHeader title={title} />
        <EmptyState>{t("extProfiler.none")}</EmptyState>
      </>
    );
  }
  const size = formatBytes(message.body_bytes, locale);
  let note = "";
  if (message.streamed) note = t("extProfiler.body.streamed");
  else if (message.binary) note = t("extProfiler.body.binary", { size });
  else if (message.cut) note = t("extProfiler.body.cut", { size });
  return (
    <>
      <SectionHeader title={title} />
      <DataTable<Header>
        label={title}
        rows={headerRows(message.headers)}
        rowKey={(row) => `${row.name}:${row.value}`}
        columns={[
          {
            key: "name",
            header: t("extProfiler.col.name"),
            render: (row) => <code>{row.name}</code>,
          },
          {
            key: "value",
            header: t("extProfiler.col.value"),
            grow: true,
            render: (row) => (
              <code style={{ wordBreak: "break-all" }}>{row.value}</code>
            ),
          },
        ]}
      />
      <p className="t-caption">
        {t("extProfiler.body.size", { size })}
        {note ? ` · ${note}` : ""}
        {message.body ? (
          <>
            {" · "}
            <CopyButton
              label={t("extProfiler.copy.body")}
              text={() => message.body ?? ""}
            />
          </>
        ) : null}
        {" · "}
        <CopyButton
          label={t("extProfiler.copy.headers")}
          text={() =>
            headerRows(message.headers)
              .map((h) => `${h.name}: ${h.value}`)
              .join("\n")
          }
        />
      </p>
      {message.body ? (
        <pre
          style={{
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
            maxHeight: "40rem",
            overflow: "auto",
          }}
        >
          <code>{readableBody(message.body)}</code>
        </pre>
      ) : null}
    </>
  );
}

export function ExchangePanel({ profile }: Readonly<{ profile: Profile }>) {
  const t = useT();
  return (
    <>
      <MessageView
        title={t("extProfiler.exchange.request")}
        message={profile.request}
      />
      <MessageView
        title={t("extProfiler.exchange.response")}
        message={profile.response}
      />
    </>
  );
}

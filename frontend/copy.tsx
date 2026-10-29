import { useT } from "@margince/frontend/app";
import { Button } from "@margince/frontend/design-system";
import { useRef, useState } from "react";
import type { Message, Query } from "./profile";

// What the profiler lets a developer take away: a statement that runs in psql,
// the request as a cURL command, a body or a token.
//
// The unit writes to the clipboard itself because core's copy control is not
// on the unit surface. This is a local developer tool; the label says when the
// browser refused, so a copy that did not land is never mistaken for one that did.

const SETTLE_MS = 1500;

type CopyState = "idle" | "copied" | "failed";

export function CopyButton({
  label,
  text,
}: Readonly<{ label: string; text: () => string }>) {
  const t = useT();
  const [state, setState] = useState<CopyState>("idle");
  const attempt = useRef(0);
  const copy = () => {
    const mine = ++attempt.current;
    const settle = (next: CopyState) => {
      if (mine !== attempt.current) return;
      setState(next);
      setTimeout(() => {
        if (mine === attempt.current) setState("idle");
      }, SETTLE_MS);
    };
    navigator.clipboard.writeText(text()).then(
      () => settle("copied"),
      () => settle("failed"),
    );
  };
  let shown = label;
  if (state === "copied") shown = t("extProfiler.copy.copied");
  if (state === "failed") shown = t("extProfiler.copy.failed");
  return (
    <Button variant="link" onClick={copy}>
      {shown}
    </Button>
  );
}

/** The statement with each `$N` replaced by its argument, ready for psql. */
export function runnableSql(q: Query): string {
  const args = q.args ?? [];
  const sql = q.sql.replace(/\$(\d+)\b/g, (placeholder, n: string) => {
    return args[Number(n) - 1] ?? placeholder;
  });
  return `${sql.trim()};`;
}

/** Every statement of a request, in the order it ran. */
export function runnableScript(queries: readonly Query[]): string {
  return queries.map(runnableSql).join("\n\n");
}

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'\\''`)}'`;
}

/** Headers curl works out for itself, which would only disagree if copied. */
const CURL_SKIPS = new Set(["content-length", "host", "connection"]);

/** The request as a cURL command against the installation at origin. */
export function curlCommand(
  origin: string,
  method: string,
  path: string,
  request?: Message,
): string {
  const parts = [`curl -X ${method} ${shellQuote(origin + path)}`];
  const headers = Object.entries(request?.headers ?? {}).sort(([a], [b]) =>
    a.localeCompare(b),
  );
  for (const [name, values] of headers) {
    if (CURL_SKIPS.has(name.toLowerCase())) continue;
    for (const value of values)
      parts.push(`-H ${shellQuote(`${name}: ${value}`)}`);
  }
  if (request?.body) parts.push(`--data-raw ${shellQuote(request.body)}`);
  return parts.join(" \\\n  ");
}

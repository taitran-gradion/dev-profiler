import { describe, expect, it } from "vitest";
import { curlCommand, runnableScript, runnableSql } from "./copy";

const query = (sql: string, args?: string[]) => ({
  sql,
  args,
  at_ms: 0,
  duration_ms: 0,
  rows: 0,
});

describe("runnableSql", () => {
  it("puts each argument in its placeholder, $10 apart from $1", () => {
    const args = ["'a'", "2", "3", "4", "5", "6", "7", "8", "9", "'ten'"];
    expect(runnableSql(query("SELECT $1, $10", args))).toBe(
      "SELECT 'a', 'ten';",
    );
  });

  it("leaves a placeholder it has no argument for", () => {
    expect(runnableSql(query("SELECT $1"))).toBe("SELECT $1;");
  });

  it("joins a request's statements in the order they ran", () => {
    expect(
      runnableScript([query("SELECT 1"), query("SELECT $1", ["NULL"])]),
    ).toBe("SELECT 1;\n\nSELECT NULL;");
  });
});

describe("curlCommand", () => {
  it("replays the request with its headers and body, quoted for a shell", () => {
    const cmd = curlCommand("http://localhost:8081", "POST", "/v1/deals?x=1", {
      headers: {
        "Content-Type": ["application/json"],
        "Content-Length": ["14"],
      },
      body: `{"n":"O'Neil"}`,
      body_bytes: 14,
    });
    expect(cmd).toBe(
      [
        `curl -X POST 'http://localhost:8081/v1/deals?x=1'`,
        `-H 'Content-Type: application/json'`,
        `--data-raw '{"n":"O'\\''Neil"}'`,
      ].join(" \\\n  "),
    );
  });
});

# dev-profiler

A request profiler for Margince, in the spirit of the Symfony profiler, built as
a Margince extension unit. It is for local development only: it keeps request
bodies, headers, cookies and SQL arguments in full, with nothing masked.

## What it shows

Open `#/ext/profiler` in the app. The list shows your own most recent requests.
Open one to see:

| Tab | What it holds |
|---|---|
| Timeline | every phase (auth, idempotency, agent gate, query check, operation hooks), the controller, and each SQL, AI, tool, Redis, HTTP and event entry on one time axis, with the function that triggered it |
| Request / Response | all headers and both bodies, the first 1 MB of each |
| SQL | each statement, its arguments, its caller, and a flag on a statement that ran more than once |
| AI | task, tier, provider, model, tokens, attempts, cache hit |
| Tools | agent tool calls, from MCP and REST |
| Cache | Redis commands and their keys |
| HTTP | outgoing calls from the webhook, web-reading and certificate-log clients |
| Events | outbox events the request raised |
| Writes, Hooks, Logs | audited writes, extension hook runs, log lines |

It can copy a statement as runnable SQL, the whole request's SQL, the request
as a cURL command, the URL, the token, a body or the headers.

Every profiled response carries an `X-Margince-Profile` header. Its value is
the request's correlation id, and `#/ext/profiler/<token>` opens it.

## How it reaches core without changing it

Most of the collection runs as overlays: the files under `backend/overlays/`
are compiled into core's own packages with `go build -overlay`. Each one wraps
a core function marked `//margince:decorate`, and `backend/overlays.lock` pins
the source hash of every wrapped function, so a change to one of them in core
stops the build until the overlay is read again.

The profiles live only in the API process's memory: a ring of the last 200
requests. A restart or **Clear my requests** empties it. The unit's own Go
package reads the ring through `expvar`, because a unit cannot import core's
internal packages.

## Install

This needs a Margince tree whose `gen-composition` supports overlays.

1. Put this repository at `extensions/profiler/` in the Margince tree.
2. Run `make composition`. It derives the manifest, checks the lock and writes
   `build/overlay/overlay.json`.
3. Start the stack with the overlay map:

   ```sh
   GOFLAGS="-overlay=$PWD/build/overlay/overlay.json" make dev
   ```

   The API compiles once, so every backend change needs this again.
4. To use **Clear my requests**, grant `ext_profiler_requests` (delete) to your
   role under Roles. No role holds it by default.

The unit name is `profiler`, so its folder must be `extensions/profiler/`.

## Known limits

- **Request scope:** only a request's own goroutine and context are profiled.
  Work a request starts on a detached context, or work the worker runs later,
  is not attributed to it.
- **Some HTTP clients:** outgoing HTTP is seen only from the three clients
  listed above. AI provider calls show on the AI tab, not the HTTP one,
  because core requires their transport to stay unwrapped.
- **Clipboard gate:** the screen writes to the clipboard itself, since
  core's copy control is not on the unit surface. Core's `clipboard-spelling`
  gate in `make check` reports this unit.

# ADR 0002: Go, Vue, Embedded UI, and SQLite WAL

- Status: Accepted
- Date: 2026-08-07

## Context

The initial controller may run on a low-resource VPS and manages only three
nodes. A PHP/Python application server plus Redis/database, or a Node.js runtime,
would add operational weight without providing useful scale.

## Decision

- Implement server and Agent in the current stable Go release.
- Use standard `net/http`, `chi`, `log/slog`, `database/sql`, generated `sqlc`
  access, and embedded migrations.
- Use pure-Go SQLite with WAL by default.
- Build the admin UI with Vue 3, TypeScript, Vite, Naive UI, and uPlot; embed its
  static output into the server binary.
- Use Node.js only during frontend builds.

## Consequences

- Production has a small process and few moving parts.
- Cross-compiling Linux amd64/arm64 is straightforward.
- SQLite limits multi-controller/high-write scale, which v1 explicitly does not
  target.
- Frontend and backend can be released together as one artifact.
- PostgreSQL support is deferred until measured requirements justify it.

## Rejected alternatives

- **PostgreSQL + Redis from day one:** unnecessary operational cost for the
  initial scale.
- **Electron/desktop controller:** cannot provide a continuously available
  subscription endpoint naturally.
- **Server-rendered HTML only:** lowers frontend tooling but makes dense node and
  user workflows less ergonomic.

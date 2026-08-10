# Testing and integration setup

How to test an application that uses this client: unit tests without a
server, and integration tests against a real DatoriumDB cluster.

## Unit tests (no server)

Everything the client calls goes through the standard library, so two
approaches cover most applications:

- **`httptest.Server`** — point `Config.EstablishmentURL` at a test server
  that returns canned establish / document responses. Good for exercising
  your repository layer against realistic envelopes.
- **Interface seam** — keep your own storage interface in app code and mock
  it; use the client directly only in the integration layer below.

For tokens in unit tests, `Config.Token` (or `datorium.StaticToken`) is all
you need — no key material involved.

## Integration tests (real server)

The repository ships a complete, copyable example:

- `test/integration/todo/docker-compose.yml` — a two-shard cluster
  (`server1` owns slots `00–7F`, `server2` owns `80–FF`), built from a
  DatoriumDB source tree.
- `start_integration_test.sh` — brings the stack up, waits for readiness,
  mints a client token, runs the `cmd/todo-integration` CLI, snapshots the
  server data dirs for inspection, and tears everything down.

Run it with a DatoriumDB checkout next to this repo (or set
`DATORIUMDB_SRC`):

```sh
./start_integration_test.sh
```

### Setting up your own application's stack

1. **Compose file.** Base it on `test/integration/todo/docker-compose.yml`.
   First server seeds the establishment and mounts a `.config` fixture dir
   (schemas, `__auth.json`); additional servers join by name/URL.
2. **Wait for readiness**, not just startup. Poll
   `GET /datoriumdb/v1/health` on each server, then require
   `GET /datoriumdb/v1/ready` to report `"ready":true` before sending
   traffic. The client's `Health`/`Ready` methods do the same probes.
3. **Mint a test token.** Use the development helper:

   ```sh
   go run ./cmd/mint-token \
     -auth path/to/.config/__auth.json \
     -key path/to/dev-signing-key.pem \
     -subject my-integration-test
   ```

   Pass it via `Config.Token` or an environment variable. These keys are
   for tests only — never reuse them in production. For long-running test
   services, a self-refreshing `TokenSource` avoids mid-run expiry; see
   [Client and config — Tokens](client.md#tokens).
4. **Point the client at the cluster.** `EstablishmentURL` is the first
   server's published address (e.g. `http://127.0.0.1:18081`). If the
   establishment advertises Docker-internal base URLs (like
   `http://server2:8080`) that your test process cannot resolve, map them
   with `Config.BaseURLRewrite`:

   ```go
   client, err := datorium.New(datorium.Config{
       EstablishmentURL: "http://127.0.0.1:18081",
       Token:            os.Getenv("DATORIUM_TOKEN"),
       BaseURLRewrite: map[string]string{
           "server2": "http://127.0.0.1:18082",
       },
   })
   ```
5. **Bind and go.** Call `client.Establish(ctx, ...)` once at startup, then
   bind collections and run your scenario. Exercise documents across both
   shard ranges so routing to the second server is actually tested.
6. **Tear down.** `docker compose down -v` removes volumes so each run
   starts from a clean establishment. Keep a unique `-p` project name per
   run if tests execute concurrently.

### What the example verifies

`cmd/todo-integration` walks an end-to-end story — typed CRUD, patches,
references, and search across both shards — and is a useful template for
structuring your own scenario binary. The wrapper script also copies each
server's `/db` tree to `test/integration/todo/analysis/` before teardown,
which is handy when debugging a failing run.

## See also

- Server-side limits and document-type rules (size caps, field
  constraints, validation errors your tests may hit) are documented in the
  DatoriumDB server documentation; see the limits/types pages there once
  published.
- [Errors](errors.md) — codes to assert against in negative tests.

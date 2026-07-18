# Testing

## Layers

| Layer | How | Requires network/Docker |
|-------|-----|-------------------------|
| Unit | `go test ./...` | No |
| Mocked HTTP | `httptest` in package tests | No |
| Contract shapes | fixtures under `testdata/contract/` | No |
| Todo Compose demo | `./start_integration_test.sh` | Yes |

Default CI runs formatting, vet, tidy, race-enabled unit tests, and builds.
The Compose demo is **opt-in** and not part of `go test ./...`.

## Running locally

```bash
go test ./... -race -count=1
./start_integration_test.sh
```

## Todo integration story

`./start_integration_test.sh` brings up a disposable two-shard DatoriumDB
cluster, runs a host-built Todo scenario through this client library, then
tears everything down. Step titles below are printed live as
`[STEP_NAME]` so you can follow progress in the terminal.

### Harness (`start_integration_test.sh`)

| Step | What happens |
|------|----------------|
| `CHECK_PREFLIGHT` | Verifies `docker`, `docker compose`, `go`, `curl`, and Docker daemon access. Resolves `DATORIUMDB_SRC` (default: sibling `../datoriumdb`) and checks that source includes recursive `FindRefFields` (needed for `Users.todoLists` array cached refs). |
| `COMPOSE_UP` | Builds image `datoriumdb:local` from that source and starts a unique Compose project with **server1** (`00-7F`) and **server2** (`80-FF`). Each server is dual-role SOT + read member for its range. Establishment config mounts an empty Todo app (Users, TodoLists, Todos + `Todos.byStatus` search). |
| `WAIT_CLUSTER` | Polls `GET /datoriumdb/v1/health` on both host ports (`18081` / `18082`) and `GET /datoriumdb/v1/ready` on server1 until the cluster is usable. |
| `MINT_TOKEN` | Signs a short-lived client JWT with the fixture Ed25519 key (`cmd/mint-token`) matching `__auth.json`. |
| `BUILD_CLI` | Compiles `cmd/todo-integration` into `.test-run/todo-integration`. |
| `RUN_SCENARIO` | Executes the live Todo CLI (steps below) with `DATORIUM_TOKEN` and host base URLs. Docker-internal `http://serverN:8080` URLs are rewritten to localhost for the host client. |
| `SNAPSHOT_DBS` | Before teardown (success or failure), replaces `test/integration/todo/analysis/server{1,2}/` with each container's `/db/` tree and writes `MANIFEST.txt`. Last run only (no history). Gitignored. |
| `COMPOSE_DOWN` | Always runs (shell `trap` on success, failure, or interrupt): `docker compose down -v --remove-orphans` for that project name. |

### Post-run DB analysis

After every run that got past `COMPOSE_UP`, the harness overwrites a fixed
host-side snapshot (previous analysis dirs are removed) so you can inspect
shard-local files without keeping Docker volumes:

```text
test/integration/todo/analysis/
  MANIFEST.txt          # project, timestamps, URLs, exit code
  server1/              # copy of server1 container /db
  server2/              # copy of server2 container /db
```

Typical contents under each `serverN/` (plain-text-json storage):

- `.config/` — local establishment copy (server1 also had the fixture bind-mount)
- collection document trees for IDs owned by that shard
- `.search/` precompiled search result files
- cache / pending work-item directories used by background agents

Browse with normal tools, for example:

```bash
ls test/integration/todo/analysis/server1
find test/integration/todo/analysis/server1 -type f | head
```

### Scenario (`cmd/todo-integration`)

The establishment starts **empty** (schemas and search definitions only; no
documents). The CLI picks deterministic document IDs that land in both shard
ranges so routing is exercised for real.

| Step | What happens |
|------|----------------|
| `WAIT_READY` | Polls readiness through the client until server1 reports `ready: true`. |
| `ESTABLISH` | `GET /establish`, caches general/servers/shardMap/schemas/searches/auth, prints establishment name and config version. |
| `PICK_IDS` | Chooses four IDs: two Users (low + high shard), one TodoList (low), one Todo (high). Logs each ID and its CRC32 slot hex. |
| `CREATING_USERS` | Creates **Ada** and **Grace** in `Users` with empty `todoLists` arrays (front-page holder for `@@` list refs). |
| `CREATING_LIST` | Creates a `TodoLists` document on the low shard titled "Ship client", with a live `@__Users__{Grace}` owner and a cached `@@__Users__{Grace}` ownerSummary. |
| `LINK_LIST_TO_USER` | Patches Grace's `todoLists` array to append `@@__TodoLists__{listLow}` (library helper `PatchDetailAppendingCachedRef`). |
| `READ_USER_FRONT_PAGE` | Reads **Users/Grace** with `cacheSummaries: true` until ordered front-page summaries show the list title (O(1) user home-page pattern; up to ~45s). |
| `PATCH_LIST_TITLE` | Patches the list title to `"Ship client v2"`. |
| `WAIT_FRONT_PAGE_UPDATE` | Re-reads **Users/Grace** every ~2s until the front-page cached list title updates (max ~15s). |
| `RESOLVE_LIVE_REF` | Reads the list and follows the live `@` owner reference to Grace on the high shard (smart-client routed read). |
| `READ_CACHED_REF` | One `cacheSummaries` read of the TodoList to create the local cache stub for Grace (may still be `#: null` until a later source write). |
| `PATCH_CACHED_TARGET` | Patches Grace (`displayName` → `"Grace Hopper"`), which re-queues cache-update work items for all read members. |
| `WAIT_CACHE_UPDATE` | Re-reads the **same TodoList** every ~2s until `cacheSummaries.Users.{Grace}.displayName` is `"Grace Hopper"` (max ~15s). |

Cache-update note: datoriumdb completes pending cache work as a no-op when the
read member has no stub file yet. So a referring `cacheSummaries` read must
happen first (stub), then a source-document write (or the earlier write is
already gone). The front-page path usually gets filled by `PATCH_LIST_TITLE`
after `READ_USER_FRONT_PAGE` creates the list stub.
| `CREATING_TODO` | Creates a `Todos` document on the high shard (`status: open`) with live + cached refs back to the low-shard list. |
| `PATCHING_TODO` | Reads the todo, RFC6902-patches `status` from `open` → `done`, asserts `versions.after` advanced, re-reads and confirms status. |
| `WAIT_SEARCH` | Runs precompiled `search Todos byStatus {status: done}`, routed to the search-result shard, polling until the todo ID appears in `matches` (change-agent indexing lag). |
| `DELETING_TODO` | Deletes the todo with the current `#` version (retries once on mismatch), then reads again and expects `documentNotFound`. |
| `PASSED` | Prints `todo-integration PASSED` when every assertion succeeded. |

### What this proves

- Bearer auth + establishment caching
- CRC32 shard routing across `00-7F` / `80-FF` with host URL rewrite
- Create / read / patch / delete over the access language
- Live `@` reference resolution by the smart client
- Cached `@@` summaries via `cacheSummaries`, including observing a referenced-document patch eventually appear on a later read of the referring document
- User front-page pattern: `Users.todoLists` as an array of cached TodoList refs; one user read yields ordered list titles (`SummariesForArrayField`)
- Precompiled search routing and result polling
- Clean Compose teardown even when a step fails

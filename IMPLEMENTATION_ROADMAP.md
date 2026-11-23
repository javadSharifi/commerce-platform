# Engineering Implementation Roadmap

**Status** · Phase 1 is open after independent review remediation on November 23, 2026. The legacy `primary_color` contract awaits approval and the full browser gate awaits P2-T03. Baseline `main` / `c39b423` plus local working-tree changes. Phases 2–7 await authorization.

## Progress summary

| Phase | Objective | Total | Not started | In progress | Blocked | Completed and verified |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| 1 | Build a reliable verification baseline | 4 | 0 | 0 | 2 | 2 |
| 2 | Enforce identity, tenant scope and production startup | 12 | 12 | 0 | 0 | 0 |
| 3 | Restore booking, capacity and payment invariants | 13 | 13 | 0 | 0 | 0 |
| 4 | Make deployment, media and background work recoverable | 8 | 8 | 0 | 0 | 0 |
| 5 | Make application routes report authoritative outcomes | 11 | 11 | 0 | 0 | 0 |
| 6 | Stabilize API contracts, templates and accessibility | 9 | 9 | 0 | 0 | 0 |
| 7 | Bound load and qualify the production release | 6 | 6 | 0 | 0 | 0 |
| **Total** | | **63** | **59** | **0** | **2** | **2** |

Use one marker per task. `[ ]` is **Not started**, `[x]` is **Completed and verified**, `[!]` is **Blocked**, and `[~]` is **In progress**. Update the summary and the task's evidence when its state changes. Evidence records the checkout SHA, environment class, commands and exit codes, required test cases actually run, acceptance evidence, and remaining limitations. Record a blocker with its owner, the missing decision or evidence, and the condition that unblocks it. Use the phase and task IDs to track remediation. Before implementation, register the approved slices in [the project checklist](docs/psychology/TASKS.md) in accordance with [the workflow](docs/WORKFLOW.md).

**Release rule.** Complete the Critical and High tasks and the target qualification gate before declaring launch readiness. Shared HTML caching requires a separate activation decision under PSY-D10. Keep it disabled in the initial release. Phase 6 product decisions apply to launch only when explicitly adopted.

## Global engineering rules

1. Preserve existing user changes, including the local visual track changes in [PROGRESS.md](docs/psychology/PROGRESS.md), [TASKS.md](docs/psychology/TASKS.md), and [ACCEPTANCE.md](docs/psychology/ACCEPTANCE.md). Keep each implementation slice reviewable. Follow the repository's five-file or approximately 300-line checkpoint. If a task lists several related callers, work through them in the specified slices and record evidence for each.
2. Start with a failing regression or an explicit reproducible acceptance scenario. Preserve tests, lint, RLS, authorization checks and database constraints. Record required test skips as missing evidence and keep the affected task open. Run the normal entry point and prove the failure path as well as the success path.
3. Derive tenant scope from a verified Host or authenticated membership. A requested clinic is a selector until membership is checked. Preserve `ENABLE` and `FORCE ROW LEVEL SECURITY`, explicit tenant predicates, least-privilege role boundaries, and composite tenant foreign keys. Use parameterized sqlc queries and verify every tenant-scoped change with tenant A and tenant B under runtime credentials.
4. Preserve integer rial amounts, trusted backend price and capacity, immutable purchase snapshots, and idempotent Verify. Add a migration for approved schema changes and preserve applied migration history. Keep generic abstractions, new packages, and new services out of these slices unless a concrete approved requirement warrants them.
5. Give every private or credential-bearing response an explicit `private, no-store` policy. Commit publication and invalidation events together. Restrict purge to verified hosts and applicable paths. Keep credentials out of code, Git, test output and logs.
6. Put fixtures and mocks in explicit test or development wiring. A production capability either works with its configured adapter or is visibly disabled under an approved feature policy. Document affected user flows and degradation behavior.
7. Record architectural and operational decisions in the project decision process. The roadmap proposes work and records current project rules. Approval for schema and policy changes, access grants, external services, DNS and deployment follows [AGENTS.md](AGENTS.md) and [WORKFLOW.md](docs/WORKFLOW.md).
8. Completion requires the acceptance criteria and all required tests. Report a passing result only with execution evidence. Preserve Go error causes for diagnostics and map them to safe HTTP or UI responses.

## Audit revalidation and coverage

The source still has the previous audit's runtime wiring at `c39b423`. The worktree includes existing documentation changes for the visual review and the PSY-7.7 / PSY-A15 track in `docs/psychology/{PROGRESS,TASKS,ACCEPTANCE}.md`. Source paths remained at the baseline. Evidence below comes from rereading the current source, migrations, contracts, tests and configuration. **Confirmed** means demonstrated by a code path, schema, or local measurement. **Conditional** means the code behavior is established and target impact depends on configuration, traffic or provider support. **Open** means target evidence or a product decision is required.

| Audit ID | Current finding and source | Disposition | Tasks |
| --- | --- | --- | --- |
| S1 | Payments uses workshop/course generated persistence models, and workshop HTTP initiates payment. `internal/payments/service.go:18`, `internal/workshops/http.go:223` | Confirmed | P3-T08 through P3-T12 |
| S2 | OpenAPI `/account/profile` differs from mounted `/account/auth/me`; public blog response also drifts from OpenAPI. `packages/contracts/psychology/openapi.yaml:165,1058`, `internal/courses/http.go:80`, `internal/blog/http.go:75` | Confirmed, with the delete example withdrawn below | P6-T01 |
| A1 | Tenant modules and the course media adapter are bound to the pool. RLS relies on the local setting in `WithTenantTx`. `cmd/server/main.go:43,175`, `internal/platform/db/store.go:25` | Confirmed | P2-T02, P2-T04 through P2-T07, P3-T01, P3-T05, P3-T07 |
| A2 | Initiation, callback and refund effects have independent commits; row locks are statement-scoped in current wiring. `internal/payments/service.go:253,664,885,1252` | Confirmed | P3-T07 through P3-T12 |
| A3 | Workshop capacity uses a process lock and count; booking splits slot, appointment and history writes. `internal/workshops/service.go:356`, `internal/booking/service.go:757,1404` | Confirmed | P3-T01 through P3-T03, P3-T05, P3-T06 |
| A4 | Health succeeds with unwired handlers, migrations are copied but unrun by Compose, provider/config wiring differs from deployment. `cmd/server/main.go:85,165`, `infrastructure/psychology/docker-compose.yml:15,32` | Confirmed | P2-T01, P2-T11, P4-T01, P7-T06 |
| C1 | Course checkout writes `initiated` to `purchases.status` which rejects it. `internal/payments/service.go:433`, `migrations/000002_create_workshops_and_payments.up.sql:44,107` | Confirmed | P3-T07 |
| C2 | Identity-wide membership list uses a pool query behind tenant-scoped RLS. `internal/auth/service.go:190`, `internal/auth/queries.sql:30`, `migrations/000001_create_core_schema.up.sql:80` | Confirmed | P2-T03 |
| C3 | Time-expired pending holds remain in the active-phone index. `internal/workshops/queries.sql:125`, `migrations/000002_create_workshops_and_payments.up.sql:78` | Confirmed | P3-T05, P4-T07 |
| C4 | Schedule replacement/slot regeneration can partially commit; slot uniqueness checks equal starts, while generation suppresses unrelated errors. `internal/booking/service.go:355,407,614`, `migrations/000005_create_appointments.up.sql:99` | Confirmed | P3-T04 |
| C5 | Workshop tracking uses a sample result and a query-string status; gateway return does not authorize the tracking read. `internal/payments/http.go:96`, `internal/workshops/http.go:223`, `apps/psychology-site/src/pages/workshops/registrations/[id].astro:11` | Confirmed | P3-T09, P5-T08 |
| C6 | Archived-course entitlements remain active, but the viewer uses a published-only syllabus endpoint. `internal/courses/admin_service.go:706`, `internal/courses/buyer_service.go:555`, `apps/psychology-site/src/pages/account/courses/[id].astro:222` | Confirmed | P3-T13, P5-T10 |
| C7 | Media quota is a check followed by insert; promotion precedes independent DB updates and deletes staging early. `internal/media/service.go:227,293` | Confirmed | P4-T03, P4-T04 |
| C8 | Appearance merges omitted fields in Go and writes both values after a non-locking read. `internal/settings/service.go:844`, `internal/settings/queries.sql` | Confirmed | P6-T08 |
| U1 | Public home and preview use demo records, and route-level fallbacks can show sample records or metrics after errors. `apps/psychology-site/src/components/HomePageContent.astro:41`, `apps/psychology-admin/src/pages/index.astro:76`, `apps/psychology-admin/src/pages/api/admin/site.ts:33` | Confirmed | P5-T01 through P5-T09 |
| U2 | SSR app configuration, browser clinic selection and JSON mutation headers diverge. `apps/psychology-admin/src/pages/workshops/[id].astro:329`, `apps/psychology-admin/src/pages/media/index.astro:5`, `internal/auth/middleware.go:80` | Confirmed | P5-T01 through P5-T05 |
| U3 | Slot requests can resolve out of order, and day mode falls back to all slots on zero matches. `apps/psychology-site/src/components/BookingFlow.tsx:120,170` | Confirmed | P5-T11 |
| U4 | Untyped template slots and duplicated card links leave route composition outside the template boundary. `apps/psychology-site/src/templates/contract.ts`, `apps/psychology-site/src/components/HomePageContent.astro:96` | Confirmed | P6-T05 |
| U5 | Catalog category links have missing targets, admin sidebar is fixed-width, and interactive field messages lack association. `apps/psychology-site/src/pages/courses/index.astro:108`, `apps/psychology-admin/src/layouts/AdminLayout.astro:36`, `packages/ui/src/interactive/Select.tsx:87` | Confirmed | P6-T02, P6-T03 |
| U6 | Secondary token pairs include 4.24:1, 3.19:1 and 3.96:1 for small labels; Editorial cascade changes the pair actually used. `packages/ui/src/styles/palettes.css`, `apps/psychology-site/src/templates/editorial/WorkshopCard.astro:23` | Confirmed at token level. Effective rendered contrast requires a browser gate. | P6-T06 |
| U7 | Civil date becomes midnight UTC, then formats in the host timezone. `packages/ui/src/interactive/date-utils.ts:48` | Confirmed by source and prior UTC/Los Angeles measurement | P6-T04 |
| P1 | Public middleware and routes resolve the site twice; sitemap calls are serial and fallback is publicly cacheable. `apps/psychology-site/src/middleware.ts:19`, `apps/psychology-site/src/pages/sitemap.xml.ts:17` | Confirmed | P5-T06, P5-T07, P7-T01 |
| P2 | Producer emits four index paths for one Host; CDN test seeds details itself; content writes lack producer coverage. `internal/settings/service.go:875`, `internal/platform/cache/cache_test.go:683` | Confirmed; actual edge staleness is conditional | P4-T05, P7-T04, P7-T06 |
| P3 | Public article SSR embeds five-minute URLs in one-hour cacheable HTML. `apps/psychology-site/src/pages/blog/[slug].astro:7,116`, `internal/media/service.go:25` | Confirmed mismatch; playback impact is conditional on caching | P7-T05 |
| P4 | Workshop N+1, independent reporting snapshots and uncapped booking bookkeeping. `internal/workshops/service.go:631`, `internal/reports/service.go:134`, `internal/booking/ratelimit.go:57` | Confirmed mechanisms; load impact unmeasured | P2-T05, P3-T01 through P3-T03, P7-T02 |
| P5 | Booking has substantial client assets and zero-JS assertions miss interactive detail scripts. `apps/psychology-site/src/pages/booking/index.astro`, `apps/psychology-site/test/acceptance-a14.test.mjs:475` | Prior build measured assets; route transfer and interaction budgets remain open | P7-T03 |
| R1 | Compose and role bootstrap contain executable credential defaults. `infrastructure/psychology/docker-compose.yml:9`, `infrastructure/psychology/initdb/01-roles.sql:4` | Confirmed in repository; deployed use unknown | P2-T01, P4-T01 |
| R2 | Token handling, logout, buyer cookie flags and OTP use are inconsistent. `internal/auth/http.go:99`, `internal/courses/http.go:340`, `internal/courses/queries.sql:79` | Confirmed | P2-T08 through P2-T10 |
| R3 | Admin middleware creates an owner on an upstream exception and settings begins with permission enabled. `apps/psychology-admin/src/middleware.ts:97`, `apps/psychology-admin/src/pages/settings.astro:50` | Confirmed UI trust defect; Go still authorizes data access | P5-T01 |
| R4 | Host sources differ, BFF mutations rewrite request metadata, and tracking access is carried in URLs. `apps/psychology-site/src/lib/site.ts:34`, `apps/psychology-admin/src/pages/api/admin/appearance.ts:66`, `apps/psychology-site/src/components/BookingFlow.tsx:213` | Confirmed code behavior; target exposure conditional | P2-T12, P5-T01, P5-T08, P5-T09, P7-T06 |
| R5 | PUT signing does not constrain declared type/size; finalization compares size only and persists declared MIME. `internal/media/storage.go:137`, `internal/media/service.go:317` | Confirmed | P4-T02 through P4-T04 |
| R6 | Outbox locks expire before purge; missing key succeeds; retention silently uses the app pool and can collide on empty phones; cleanup jobs lack wiring. `internal/platform/cache/outbox.go:79`, `internal/platform/cache/worker.go:48`, `internal/platform/retention/cleaner.go:32,151`, `cmd/server/main.go:230` | Confirmed | P4-T05 through P4-T07 |
| R7 | Restore ignores `pg_restore` failure and verifies only tables that exist. `infrastructure/psychology/scripts/restore.sh:24` | Confirmed | P4-T08 |
| R8 | Approved target ingress, origin isolation, TLS, forwarded-host behavior, log redaction and cache bypass lack deployment evidence. `infrastructure/psychology/Caddyfile:3` | Open target verification | P2-T12, P7-T06 |
| D1 | Go/SSR acceptance uses fixtures outside production wiring; implicit live DSN can seed a production-shaped DB; mocked CDN request proves adapter shape. `internal/testsupport/acceptance_a13_test.go:385`, `internal/platform/cache/cache_test.go:487`, `apps/psychology-site/test/booking-redesign.test.mjs:220` | Confirmed | P1-T02 through P1-T04, P7-T06 |
| D2 | Make and the pnpm verify script test before building, and tests launch `dist`. `Makefile:11`, `package.json:13`, `apps/psychology-admin/test/ssr-render.test.mjs:5` | Confirmed | P1-T01 |
| D3 | Palette and template data are repeated; resolver ignores persisted version. `packages/ui/src/template-catalog.ts`, `packages/ui/src/palette-catalog.ts`, `internal/settings/service.go:34`, `apps/psychology-site/src/templates/registry.ts:27` | Confirmed; future version semantics require an explicit decision | P6-T06, P6-T07 |
| D4 | Single configured appearance owner is intentional for the pilot and a delegation limitation for future self-service. `internal/settings/http.go:115`, `docs/psychology/modules/site-appearance.md:30` | Confirmed behavior; future capability decision open | P2-T11, P6-T09 |
| N1 | Editorial declares the same CSS color properties as the palette and appears later in the existing built stylesheet. Shell and route both supply horizontal gutters. `apps/psychology-site/src/layouts/BaseLayout.astro:2`, `apps/psychology-site/src/templates/editorial/theme.css:2`, `apps/psychology-site/src/templates/editorial/Shell.astro:13` | Newly confirmed cascade and composition defect | P6-T05, P6-T06 |
| N2 | One global merchant adapter is used despite the tenant merchant requirement in PSY-D04. `cmd/server/main.go:187`, `internal/payments/zarinpal_provider.go:35` | Newly confirmed financial scope mismatch | P3-T08, P7-T06 |
| N3 | The booking form explicitly invites a reason for seeking treatment, and the API stores it. PSY-D02 prohibits collecting it. `apps/psychology-site/src/components/BookingFlow.tsx:527`, `internal/booking/http.go:234`, `internal/booking/service.go:815` | Newly confirmed data minimization defect | P3-T01, P4-T06, P5-T11 |
| N4 | Go course responses use `cover_media_id` and the storefront reads `cover_url`. Public blog returns an array while OpenAPI declares `items/meta`. `internal/courses/admin_service.go:119`, `apps/psychology-site/src/pages/courses/index.astro`, `internal/blog/http.go:88`, `packages/contracts/psychology/openapi.yaml:165` | Newly confirmed payload mismatches | P5-T06, P5-T07, P6-T01 |

Paths beginning with `cmd/`, `internal/` or `migrations/` in this register are relative to `apps/psychology-api`. The audit's claim that blog deletion returns `204` is withdrawn. The handler and contract both return `200` JSON. The client helper always parses JSON, but this review did not establish a currently mounted `204` operation affected by that behavior. Cover empty successful responses as a client contract regression test. The remaining prior findings are tracked above. Target facts such as active caching, provider capability, actual credential usage and live throughput require evidence from the approved environment.

## Execution and evidence model

- **Start first** · P1-T01 and P1-T02 can start independently. P2-T01 can proceed alongside them with coordination on the fixture role contract. P1-T03 follows P1-T02. P1-T04 follows P1-T01 and can start with SSR fixtures while the Go harness is built.
- **Exclusive ownership** · Serialize changes to `cmd/server/main.go` on the integration branch. Modules can prepare separately, then land their small wiring change one at a time. Serialize auth/clinic transport changes across Go, Astro middleware and the admin BFF. Serialize payment initiation, callbacks and refunds P3-T07 → P3-T08 → P3-T09 → P3-T10 → P3-T11 → P3-T12. Schedule generation waits for booking transitions. Workshop capacity writes wait for registration.
- **Safe parallel tracks** · After P2-T02, blog, report and media query conversions are independent until the wiring change. After the course scope boundary, media quota and booking invariants can progress independently, with serialized DB migration integration. P4 recovery work and P5 route work can run concurrently once their task-specific dependencies pass. P6 design and date tasks can proceed against stable fixture routes while independent operations finish. Route and shared UI tasks that touch the same page land in sequence.
- **Integration stop points** · Verify scope before enabling writes on a converted module. Verify payment and capacity invariants before payment-facing browser acceptance. Verify SQL and storage promotion before scheduling cleanup. Verify outbox producer and consumer semantics, playback expiry and target isolation before any shared HTML activation. P7-T06 may run with HTML caching off after release-critical checks.
- **Migration coordination** · Assign migration sequence numbers only when integrating. Use existing migration and sqlc conventions. Keep reader/writer compatibility during roll-forward, check existing data before adding constraints, and use backup/restore or a tested rollback path when data mapping is non-reversible.

### Verified commands and test lanes

Planning recheck on November 23, 2026 · `pnpm lint` passed. `make verify-psychology` reached the Node SSR tests after Go verification and TypeScript checks. It exited at the SSR step because this environment denied loopback listeners (`listen EPERM`). The Go output reported eight skipped live cases. Treat this as incomplete runtime verification. The prior audit's fresh frontend build and 60 passing Node tests used a session with loopback permission. No database fixture or target provider was exercised for this roadmap.

Run `make verify-psychology` and `pnpm lint` from the root for completed slices. Both `make verify-psychology` and `pnpm verify:psychology` now build before typecheck and SSR tests. The workspace pins pnpm 10.12.1 in `package.json`. Targeted commands include `make verify-go`, `pnpm typecheck`, `pnpm --filter psychology-site test`, `pnpm --filter psychology-admin test`, `pnpm --filter @psychology/ui test`, and `pnpm --filter @psychology/api-client test`. The focused Go lane runs from `apps/psychology-api`, for example `go test -count=1 ./internal/booking ./internal/workshops ./internal/payments`. Run Go race checks where they measure shared process state. Cross-process correctness requires PostgreSQL tests with independent connections and processes.

Run `make test-db` from the root for mandatory uncached migration, isolation, outbox and executable checks. The fixture runner supplies `TEST_ADMIN_DATABASE_URL`, `TEST_APP_DATABASE_URL`, `TEST_WORKER_DATABASE_URL` and a separate `TEST_MIGRATION_DATABASE_URL`. It requires nine named live tests to pass. The general Go lane reports these tests as skipped when run without fixture connections. The root Makefile defines `test-db`. The documented standalone `migrate-up` and `migrate-down` targets remain proposals. `sqlc generate` is the documented generator command from `apps/psychology-api` when sqlc is installed. Any new browser, ingress, concurrency, recovery or contract case named below is a required deliverable unless identified as an existing test.

**Phase completion** · Verify all tasks in that phase, plus the listed phase gate. Store evidence in the relevant PR or local verification record and update this roadmap. An unsigned provider assumption, missing evidence, skipped mandatory lane or unapproved product change cannot close its dependent task.

## Phase 1 · Build a reliable verification baseline

**Objectives and problems** · Make a passing check identify the code and execution path it verified. Address stale SSR artifacts, ambient database targets, hand-built Go acceptance wiring and browser behavior gaps (D1, D2).

**Sequence and outcome** · This phase needs no prior implementation. Establish harnesses before rewriting invariants so regressions are measurable. Exit with current-build frontend tests, a disposable runtime-role database, an assembled executable smoke suite, and a browser harness that can run against it.

### P1-T01 · Build the exact frontend artifacts tested

- [x] **High** · Objective — make both root verification entry points test the current build.
- **Cause and files** · `Makefile`, `package.json` and frontend `test/ssr-render.test.mjs` use a build-after-test order.
- **Changes and decision** · Reorder both verification entry points. Keep package test commands and their assertions. Document the pinned pnpm version and keep generated artifacts out of review.
- **Dependencies and regression** · None. Check workspace package build order, especially `@psychology/ui` and `@psychology/api-client`.
- **Tests, acceptance and Done** · Run both root verification entry points in a clean disposable checkout with no app `dist`. Show SSR processes load the newly built entry points. Required DB checks still require the Phase 1 database lane. Done when build-before-test ordering is proven by logs and lint passes.

### P1-T02 · Provision an isolated PostgreSQL test lane

- [x] **High** · Objective — make migration and runtime-role checks deterministic.
- **Cause and files** · `apps/psychology-api/migrations/migration_test.go`, `internal/testsupport/acceptance_a13_test.go`, `internal/settings/isolation_test.go`, and `infrastructure/psychology/initdb/01-roles.sql` rely on separate DSN conventions and implicit fallback.
- **Changes and decision** · Create one supported local fixture entry point and document it. Provision a disposable database with separate migration, runtime and worker roles. Require explicit connection information, check database identity and DB role before seeding, and remove the implicit A13 database fallback. Preserve voluntary unit-only runs and make missing required live tests fail the acceptance lane.
- **Dependencies and regression** · None. Coordinate role names with P2-T01. Protect developer databases and verify cleanup executes only against the named fixture.
- **Tests, acceptance and Done** · Run each migration up/down/up and the full stack up/down/up against `public` in a second disposable database. Verify core RLS after reapplication, scoped data access as `app_user`, the worker grants and policy rollback, and the unchanged runtime fixture. Run settings isolation, A13 and applicable outbox tests uncached. Show disallowed DSNs and Docker endpoints fail before provisioning or seeding, and that interrupting the live runner removes its owned container and API process. Required skips fail the lane.

### P1-T03 · Exercise the production composition root locally

- [!] **High** · Objective — verify the actual API executable against migrated PostgreSQL and two verified Host records. The executable checks passed. The legacy response-field decision below remains open.
- **Cause and files** · `apps/psychology-api/cmd/server/main.go`, `cmd/server/main_test.go`, and `internal/testsupport/acceptance_a13_test.go` contain divergent runtime and test wiring.
- **Changes and decision** · Launch the real server in explicit fixture mode. Use bounded readiness waits, distinct dynamic test ports where supported, captured stderr and process cleanup. Start with health/settings and reusable host, identity and transaction fixtures. Add each business route's gate in its owning task.
- **Dependencies and regression** · P1-T02. Preserve existing mock and live isolation assertions. Avoid starting fixture providers outside the test environment.
- **Tests, acceptance and Done** · Launch an API with two hosts, verify distinct settings, invalid/unknown host handling and wrong-membership denial. Compare Clinic B's persisted settings and complete outbox rows before and after the denied write. Confirm tests assert the runtime role and exercise the real constructor. Resolve the legacy `primary_color` contract and add the approved response regression before closing the review finding.

### P1-T04 · Establish a browser acceptance lane against current builds

- [!] **High** · Objective — verify browser actions and computed UI behavior using actual SSR assets. An authenticated session without a clinic cookie cannot discover its clinic until P2-T03.
- **Cause and files** · `apps/psychology-site/test/{acceptance-a14,booking-redesign}.test.mjs` and `apps/psychology-admin/test/{admin-pages,settings-two-clinics}.test.mjs` assert mostly HTML and fixture HTTP behavior.
- **Changes and decision** · Keep those tests. Document a supported desktop light browser run against the built apps, with fixture hosts and captured network, console and UI assertions. Start with an existing browser tool or an approved harness. Add reproducible failure checks as each route is remediated.
- **Dependencies and regression** · P1-T01 and the verified executable harness from P1-T03. Final authenticated browser acceptance also requires P2-T03 and a new run without a preselected clinic. P2-T03 uses the executable harness and can be authorized independently of the P1-T04 browser sign-off. Keep browser fixture state isolated per run.
- **Tests, acceptance and Done** · Exercise real navigation, exact computed palette colors, an interactive action, and authenticated clinic discovery from the session alone. Capture the request and resulting UI state. Show the deliberate failure probe exits nonzero and that timeout removes Chrome, both SSR processes and the profile. Close after the full lane passes.

**Phase verification** · Root build, tests and lint. Mandatory uncached migration/RLS lane. Executable two-host smoke. Maintained browser fixture instructions and recorded checks.

### Phase 1 execution record · November 23, 2026

This table records the initial execution on `c39b423` with the Phase 1 working-tree implementation. The independent review reopened the items tracked below.

| Task | Verification and actual result |
| --- | --- |
| P1-T01 | Before the change, `pnpm verify:psychology` in a disposable snapshot without `dist` failed with TS2307 for the unbuilt workspace packages. After the change, `pnpm verify:psychology` and `make verify-psychology` each passed from a separate snapshot without `dist`. Both built before testing and passed all 60 Node tests. Workspace dependencies were reused from the installed copies. Root `make verify-psychology` and `pnpm lint` also passed. The ordinary root Go lane reported nine intentional live skips; the strict database lane passed those tests separately. |
| P1-T02 | Before the change, A13 reported a passing Go invocation with a skipped test when the strict lane was given an unreachable DSN. Missing and disallowed fixture DSNs now exit nonzero before seeding. `make test-db` passed all nine required live tests, including migration up/down/up, settings isolation, A12/A13 and outbox processing under verified roles. All generated containers were cleaned up. |
| P1-T03 | The executable test initially failed because the API did not report an actual loopback listener at port zero. It now builds and launches the API and logs its PID, listener and checkout. The live test passed health, two distinct verified clinic profiles, unknown and unverified host responses, authorized content updates that preserve the active appearance values, and cross-membership read/write denial. The old post-update assertion checked the retired `primary_color` appearance contract. The revised assertion checks the submitted title and phone plus the stored template and palette. |
| P1-T04 | `CHROME_BIN=/path/to/chrome make test-browser` passed against freshly built Astro apps, the compiled Go API and disposable PostgreSQL. At 1440 × 900 in desktop light mode, Chrome checked distinct clinic titles and selected palettes, a computed CSS token, the real login POST with HTTP 401, visible inline feedback and no uncaught browser script exception. Running with `PHASE1_BROWSER_FAIL_PROBE=1` exited nonzero on the deliberate HTTP 200 expectation and printed the actual 401. Sanitized browser evidence is printed with the Go test output. |

**Run locally** · Use Docker with a local `postgres:16-alpine` image for `make test-db`. The fixture has a random database name, generated local credentials, a loopback-only Docker port and disposable in-container storage. The server verifies the database marker, actual role and forced RLS before any test seeds records. For the browser lane, supply the absolute executable path of an installed Chrome or Chromium as `CHROME_BIN`. The smoke uses a temporary browser profile and dynamic loopback listeners and tears them down on completion.

**Files created** · `scripts/test-db.mjs`, `scripts/test-browser.mjs`, `apps/psychology-api/cmd/server/executable_test.go`, `apps/psychology-api/internal/testsupport/dbfixture/connect.go`.

**Files modified** · `Makefile`, `package.json`, `apps/psychology-api/cmd/server/{main.go,main_test.go}`, `apps/psychology-api/internal/settings/isolation_test.go`, `apps/psychology-api/internal/testsupport/{acceptance_a12_test.go,acceptance_a13_test.go}`, `apps/psychology-api/migrations/migration_test.go`, `docs/psychology/{TASKS.md,PROGRESS.md}`, and this roadmap. No files deleted.

**Known limits and follow-up** · Direct Astro-to-Go public SSR returned 404 in this fixture. Local probes on Node 22 and 24 showed that native fetch sends the upstream origin as `Host` while preserving `X-Forwarded-Host`, and Go resolves `Host`. The passing browser lane uses a fixture-only relay for `GET /v1/public/site` with an allowlist of the two seeded hosts. Direct production forwarding and its trust model still require P2-T12 verification. This run used Node 24.22.1, Go 1.27.4 and installed pnpm 9.15.9 because Corepack could not download the pinned pnpm 10.12.1 from the registry. The pin is unchanged. External providers, live production ingress and production data were outside these checks.

**Approvals** · No additional approval is needed for the local Phase 1 fixtures. Starting Phase 2 requires explicit user instruction. Decisions about production identity, roles, ingress, providers and releases remain with their owners in the corresponding gates.

### Phase 1 independent review remediation · November 23, 2026

- **Migration isolation** · The round-trip test uses the second marked fixture database and its `public` schema. It exercises individual migration reversals and the full reverse sequence, checks forced RLS and effective grants, and reads settings and outbox rows as `app_user` to verify tenant isolation after reapplication. The runtime fixture's schema remains separate. No existing migration changed.
- **Resource cleanup and endpoint** · The runner resolves the Docker context before creating resources, supports a local Unix socket and pins that endpoint for cleanup. An ordinary TCP or SSH endpoint is rejected. Each child is supervised as a Unix process group. Browser cancellation reaches the Node supervisor before the Go timeout escalates. The runner deletes its labeled container and scratch directory. An externally forced kill of the supervisor or a daemon outage can still prevent cleanup.
- **Wrong-membership write** · The live executable returns `403` to the unauthorized update. The test records Clinic B's actual settings and every persisted outbox field before and after that request under `app_user` and asserts equality. It waits for setup events to settle before taking the first snapshot.
- **Browser blocker and owner** · `GET /v1/admin/auth/me` returned `200` and zero memberships for a valid session with a seeded clinic. `ListMembershipsByUserID` uses the pool without tenant scope in `apps/psychology-api/internal/auth/service.go`. The forced membership policy in `000001_create_core_schema.up.sql` filters on `app.tenant_id`, so the list is empty and Astro cannot select the clinic for `/settings`. The separate preselected-clinic scenario verifies appearance but cannot satisfy discovery. The real discovery assertion stays mandatory and the lane exits nonzero. The owner is P2-T03. Resume this gate after the authorized membership-discovery change and a fresh `make test-browser` run.

#### `primary_color` decision awaiting approval

PSY-D10 assigns layout to `template_id` and colors to `palette_id`. The public rendering and appearance publish contract exclude `primary_color`. `PublicSiteProfile` and `SiteSettingsResponse` still require `primary_color` in OpenAPI. `PUT /v1/admin/site` rejects it as an input, while `UpdateProfile` defaults the absent field to `#0284c7` and `UpsertSiteProfile` overwrites the stored value. The appearance decision does not specify that legacy reset. No handler, query, schema or OpenAPI behavior changed in this remediation.

| Option | Response contract and regression |
| --- | --- |
| **A · Preserve and deprecate (recommended)** | Keep the legacy read-only response string in the current API. Retain its stored value across content and appearance updates and use the existing default for newly created records. Mark the response property deprecated in OpenAPI and keep it out of palette selection and rendering. Seed a nondefault color, update title/phone and appearance in separate calls, and assert the original color in both admin and public responses alongside the independently updated template and palette. Verify that a client-supplied color is still rejected. |
| **B · Document the current reset** | Keep the response field and explicitly document that a content update sets it to `#0284c7`. Assert the before-and-after color plus the unaffected template and palette in the executable smoke. This preserves current wire behavior and discards old values on subsequent content saves. |
| **C · Retire in a versioned contract** | Inventory external consumers, approve a compatibility window, and remove the field only in the new contract. Assert that the old API still returns a string and the new API omits it while preserving appearance behavior. |

The project owner needs to approve one option before behavior or the public contract changes. Until then, P1-T03's executable checks remain verified and the review item stays blocked.

#### Local verification and changed files

Run `make verify-psychology`, `pnpm verify:psychology`, `pnpm lint`, `make test-fixture`, and `make test-db`. From built SSR assets, `CHROME_BIN=/absolute/path/to/chrome make test-fixture-live` verifies cancellation. Run `CHROME_BIN=/absolute/path/to/chrome make test-browser` for fresh builds and full browser acceptance. The failure control uses `PHASE1_BROWSER_FAIL_PROBE=1` with that command.

The verification snapshot starts at `c39b423` and includes the Phase 1 worktree changes and the existing PSY-7.7.2–3 appearance changes needed by its browser tests. It excludes the concurrent PSY-7.7.4 work. Installed dependencies were reused under Node `24.22.1`, pnpm `9.15.9`, Go `1.27.4`, and local OrbStack PostgreSQL 16. Corepack could not download the pinned pnpm `10.12.1` due to a registry connection timeout.

| Command | Actual result |
| --- | --- |
| `make verify-psychology` | Passed in the verification snapshot after rebuilding both SSR apps from absent `dist` directories. Go vet and unit tests passed, all 60 workspace Node tests passed, and the nine harness tests passed. The shared run failed at the concurrent home/preview assertion (`404` instead of `200`). |
| `pnpm verify:psychology` | Passed in the verification snapshot with all 60 workspace tests. The shared run recorded 59 passes and the same concurrent home/preview failure. |
| `pnpm lint` | Passed in the shared checkout and the verification snapshot. |
| `make test-fixture` | Passed all nine Node harness tests, including local and remote endpoint cases and process-group interrupt and timeout cases. |
| `CHROME_BIN=… make test-fixture-live` | Passed both live cancellation tests in the shared checkout and the verification snapshot. `SIGINT` removed the owned PostgreSQL container, API process and workspace. Browser timeout stopped Chrome and both SSR process groups and deleted the Chrome profile. |
| `make test-db` | Passed all nine mandatory tests in the verification snapshot and on the final shared-checkout rerun, including the isolated migration round trip and `denied_settings_write_has_no_side_effects`. An earlier shared run failed in the concurrent public home profile and portrait subtest. |
| `go test -count=1 -run '^TestMigrationUpDownUpLive$' ./migrations` with the strict fixture flag and a missing or disallowed DSN | Both controls exited `1` at the fixture validation guard before seeding. |
| `CHROME_BIN=… make test-browser` and the same browser stage on the snapshot's freshly built assets | The shared run exited `2` from Make and the isolated runner exited `1`. Both recorded `200` with zero memberships from `/v1/admin/auth/me` and failed the mandatory discovery assertion. |
| `PHASE1_BROWSER_FAIL_PROBE=1` browser stage on the snapshot | Exited `1` at the deliberate login check with actual `401` and expected `200`. |

Evidence is retained in the local `/tmp/iran-remediation-*.log` files. Separate attempts on the moving shared checkout also recorded a `404` from the concurrent home/preview acceptance test and temporary Go build and migration guard failures while the related source was being edited. Those files and tests were preserved.

**Created** · `scripts/fixture-process.mjs`, `scripts/test-phase1.test.mjs`, `scripts/test-phase1-live.test.mjs`.

**Modified** · `Makefile`, `scripts/test-db.mjs`, `scripts/test-browser.mjs`, `apps/psychology-api/internal/testsupport/dbfixture/connect.go`, `apps/psychology-api/migrations/migration_test.go`, `apps/psychology-api/cmd/server/{main_test.go,executable_test.go}`, this roadmap and `docs/psychology/PROGRESS.md`. No files deleted.

## Phase 2 · Enforce identity, tenant scope and production startup

**Objectives and problems** · Restore data access under the real app role and remove runtime ambiguity around identities, credentials, adapters and ingress. Address A1, A4, C2, R1, R2 and the host part of R4/R8.

**Sequence and outcome** · Use Phase 1's independently observable database and executable lanes before exposing corrected handlers. Ship role/bootstrap and transaction infrastructure ahead of business workflows. The phase establishes stable plumbing. Booking, workshops and payments complete their scoped wiring with Phase 3 invariants.

### P2-T01 · Require injected role credentials

- [ ] **Critical** · Objective — remove executable database and session-secret defaults from deployment paths.
- **Cause and files** · `infrastructure/psychology/{docker-compose.yml,.env.example,initdb/01-roles.sql}` supplies fixed credentials. Existing volumes bypass initial bootstrap.
- **Changes and decision** · Require secret injection and keep examples declarative. Preserve role separation and runtime RLS. Plan a separately authorized rotation for existing installations with overlapping validity or a maintenance window appropriate to the platform.
- **Dependencies and regression** · Coordinate with P1-T02. Rotation is conditional on an identified deployment and operator approval. Existing volumes, worker access and backup credentials need explicit verification.
- **Tests, acceptance and Done** · Validate Compose with synthetic credential inputs, show missing required values fail, then confirm an isolated fresh setup has least-privilege roles and no reusable defaults. Done when bootstrap and rotation procedures are documented and actual deployment status is recorded.

### P2-T02 · Define the mandatory tenant unit of work

- [ ] **Critical** · Objective — make `WithTenantTx` the reusable scope for tenant reads and writes.
- **Cause and files** · `apps/psychology-api/internal/platform/{db/store.go,tenantctx/tenantctx.go}` provides scope while `cmd/server/main.go` wires raw pool clients.
- **Changes and decision** · Define a narrow transaction entry point used by sibling repositories, preserve transaction-local tenant configuration, and reject missing, zero or conflicting tenant scope. Keep global lookup and worker dependencies separate. Avoid nested per-query transactions in one use case.
- **Dependencies and regression** · P1-T02. Preserve settings transactions and prevent scope leakage across pooled sessions.
- **Tests, acceptance and Done** · Verify two tenant transactions on a reused pool, rollback of a compound operation and denial outside scope using a verified role. Done when the interface and transaction contract are proven and converted modules have a single supported pattern.

### P2-T03 · Restore authenticated clinic discovery

- [ ] **High** · Objective — let `GET /v1/admin/auth/me` enumerate the authenticated user's actual memberships under production permissions.
- **Cause and files** · `internal/auth/{service.go,queries.sql,middleware.go}` uses `ListMembershipsByUserID` outside tenant scope; `000001_create_core_schema.up.sql` enforces tenant RLS.
- **Changes and decision** · Choose an approved least-privilege identity lookup, such as a narrowly granted read operation bound to the authenticated subject. Preserve membership verification for candidate clinic IDs and check the active-clinic rule against PSY-D01. Keep auth principal and active clinic separate.
- **Dependencies and regression** · P2-T02, P1-T03, approval for any lookup role or migration. Risk includes exposing unrelated memberships or blocking multi-clinic administrators.
- **Tests, acceptance and Done** · Use existing account/membership tests and add real-role tests for a user with two active memberships, revoked membership and unrelated clinic. Done when executable `/me` and protected data calls enforce these boundaries.

### P2-T04 · Convert blog to transaction-bound queries

- [ ] **Critical** · Objective — make public reads and admin publication work under RLS.
- **Cause and files** · `cmd/server/main.go`, `internal/blog/{service.go,http.go,queries.sql}` bind `NewService` to a pool.
- **Changes and decision** · Construct blog queries inside the resolved tenant transaction. Preserve draft visibility, slug uniqueness and the existing array response for current callers until the contract decision. Keep cache producer changes in P7-T04.
- **Dependencies and regression** · P2-T02, P1-T03. Scope middleware, existing draft routes and admin owner/content-manager privileges.
- **Tests, acceptance and Done** · Use the existing blog tests and extend PSY-A05 coverage through the executable. Confirm fixture A publish/read/unpublish, draft rejection and tenant B denial. Done after runtime paths and rollback pass.

### P2-T05 · Convert reports to one scoped read boundary

- [ ] **Critical** · Objective — return real owner metrics with a consistent tenant snapshot.
- **Cause and files** · `cmd/server/main.go`, `internal/reports/{service.go,queries.sql,http.go}` issue separate aggregate reads through the pool.
- **Changes and decision** · Execute `GetTenantMetrics` in a tenant read transaction with an appropriate consistent snapshot. Preserve owner-only access and the zero-PII response. Keep query optimization in P7-T02.
- **Dependencies and regression** · P2-T02, P2-T03, P1-T03. Preserve money totals and empty-metric semantics.
- **Tests, acceptance and Done** · Run existing report tests, then verify seeded totals, disallowed role, wrong clinic and a coherent read during a concurrent write. Done when the mounted metrics route returns correct totals as `app_user`.

### P2-T06 · Scope media persistence and expose transaction-compatible validation

- [ ] **Critical** · Objective — make media read and metadata operations tenant-scoped.
- **Cause and files** · `cmd/server/main.go`, `internal/media/{service.go,queries.sql,http.go}` use an unscoped query object, and the main-owned course adapter repeats it.
- **Changes and decision** · Bind media reads/writes to tenant transactions. Provide a narrow validation operation callable with a transaction from another use case. Preserve verified public/private access class checks. Keep HTTP/object-store effects outside DB locks.
- **Dependencies and regression** · P2-T02, P1-T03. Course wiring in P2-T07 must share its caller's scope. Media reserve/finalize writes remain a release gate until P4-T02 through P4-T04 pass.
- **Tests, acceptance and Done** · Run existing media tests and add real-role listing, media ID ownership and verified-access checks. Done when the executable can read authorized metadata and refuses other-tenant references without a scope bypass.

### P2-T07 · Scope course catalog, buyer data and lesson validation

- [ ] **Critical** · Objective — bind course and buyer tables to authorized tenant transactions.
- **Cause and files** · `cmd/server/main.go`, `internal/courses/{admin_service.go,buyer_service.go,buyer_middleware.go,queries.sql}` receive pool queries and use the separate media adapter.
- **Changes and decision** · Convert public catalog/admin reads, lesson validation and buyer queries in separate small slices. Put tenant filter and session principal on the same data path. Use P2-T06 for media validation. Write signed URL responses outside the transaction after an authorized lookup.
- **Dependencies and regression** · P2-T02, P2-T06, P1-T03. Preserve archived streams, public preview access and existing session selection. Keep checkout behind Phase 3 gates.
- **Tests, acceptance and Done** · Verify real-role CRUD/read authorization, lesson media ownership, preview-versus-entitlement, and buyer tenant isolation using existing course regression files plus the executable. Done when every owned scoped path has an isolation assertion.

### P2-T08 · Normalize panel session issuance and revocation

- [ ] **High** · Objective — establish secure browser sessions and real logout.
- **Cause and files** · `internal/auth/{http.go,middleware.go,service.go}`, `apps/psychology-admin/src/pages/login.astro`, `src/layouts/AdminLayout.astro` expose a JSON browser token and use inconsistent logout credentials.
- **Changes and decision** · Resolve auth credential once and revoke that same session. Use the host-only Secure/HttpOnly cookie contract for browsers, remove the JavaScript cookie fallback, and give any supported Bearer consumer a documented compatibility path. Fail visibly if revocation fails.
- **Dependencies and regression** · P2-T03 and the session compatibility decision if external Bearer consumers exist. Coordinate with P5-T01. Existing sessions, auth errors and cookie path clearing need checks.
- **Tests, acceptance and Done** · Verify browser login and logout, denied credentials, no session token in browser-readable response storage, failed revocation, and Bearer logout when supported. Done when API/client contract and browser tests agree.

### P2-T09 · Consume buyer OTP exactly once

- [ ] **High** · Objective — issue a buyer session only after one successful tenant-scoped OTP consume.
- **Cause and files** · `internal/courses/{buyer_service.go,queries.sql,buyer_test.go}` reads then consumes by ID without a predicate for active unconsumed state.
- **Changes and decision** · Bind attempt accounting, guarded consume, buyer lookup/create and session issuance to one transaction. Preserve expiry and attempt limits. Invalidate superseded codes through an explicit error-handled update. Keep delivery outside the transaction with visible failure state.
- **Dependencies and regression** · P2-T07, P1-T02. Preserve issuance retry, blocked accounts and latest-code behavior.
- **Tests, acceptance and Done** · Run the existing buyer suite and add concurrent valid verification, incorrect attempts, expired/replaced code and failure after consume. Done when only one committed session is issued per challenge and retries have deterministic outcomes.

### P2-T10 · Align buyer cookie and logout behavior

- [ ] **High** · Objective — keep the buyer session isolated, secure, revocable and private.
- **Cause and files** · `internal/courses/{http.go,buyer_middleware.go,buyer_service.go}` issues a cookie without `Secure`, returns the token, and ignores logout errors. Frontend account and detail scripts consume the browser contract.
- **Changes and decision** · Use safe production cookie flags and explicit no-store on auth responses. Separate browser credential handling from approved non-browser access. Clear and revoke consistently, and handle expected failure responses in existing account consumers.
- **Dependencies and regression** · P2-T07, P2-T09, P2-T08 cookie convention. Cover buyer/admin session independence and cross-host browser behavior.
- **Tests, acceptance and Done** · Check production cookie attributes, logout revocation, expired session, same-host account flow and two-host isolation. Done when browser and API tests show successful and failed logout accurately.

### P2-T11 · Validate configuration and declare provider availability

- [ ] **High** · Objective — make startup reject invalid enabled capabilities and expose reliable readiness.
- **Cause and files** · `cmd/server/main.go`, `infrastructure/psychology/{docker-compose.yml,.env.example}`, `internal/settings/http.go` allow missing dependencies, inconsistent env keys and implicit mocks.
- **Changes and decision** · Validate DB and schema readiness, normalize documented S3 and worker settings, and preserve the current appearance policy with explicit configuration. Keep liveness independent. Require explicit sandbox/mock mode for test wiring. Under PSY-D02/D08, obtain approval for buyer access delivery or define visible course purchase unavailability at launch. Never substitute a fake sender for available production login.
- **Dependencies and regression** · P2-T01, P1-T03. Document the configuration interfaces now. P4-T01 and P3-T08 integrate the migration and tenant merchant capabilities under their own gates. Guard against disabling legitimate local tests and leaking secret config in diagnostics.
- **Tests, acceptance and Done** · Table-test absent, incomplete and valid synthetic config. Exercise readiness with missing DB, fixture DB, wrong migration and unavailable providers. Done when enabled routes and capability messages reflect real configuration, and all dependent product decisions are linked.

### P2-T12 · Normalize host trust at ingress and verify private cache rules locally

- [ ] **High** · Objective — give public SSR, sitemap and API the same verified host and private response contract.
- **Cause and files** · `infrastructure/psychology/Caddyfile`, `apps/psychology-site/src/lib/site.ts`, `src/pages/sitemap.xml.ts`, `internal/domain/{resolver.go,queries.sql}` use different host sources and hand-written route policies.
- **Changes and decision** · Define canonical trusted host handling with approved proxy headers and ingress-only origin access. Make direct runtime listeners use an explicit local fixture policy. Add canonical host and private-route classification tests. Preserve public alias behavior and keep shared HTML disabled.
- **Dependencies and regression** · P1-T03. Coordinate the sitemap change with P7-T01 and admin CSRF with P5-T01. Host normalization, aliases and callback URLs can regress.
- **Tests, acceptance and Done** · Verify both registered hosts, alias, unknown/unverified host, inconsistent forwarding metadata, private 2xx/4xx/5xx and gateway callback routing through local ingress. Done when normalized scope and no-store rules have executable evidence. Target verification remains P7-T06.

**Phase verification** · Mandatory executable and live role lanes, tenant-isolation matrix for converted modules, successful and denied browser auth, cookie/header review, and configuration readiness checks. No tenant write is released ahead of its applicable Phase 3 or Phase 4 invariant gate.

## Phase 3 · Restore booking, capacity and payment invariants

**Objectives and problems** · Ensure a reservation, payment, refund or course entitlement has one authoritative durable outcome. Address A1 for transactional modules, A2, A3, C1, C3–C6, S1, N2 and N3.

**Sequence and outcome** · Verify reservation and course boundaries before payment reconciliation can rely on them. Booking and workshop tracks can run independently. Keep the shared payment service serialized after workshop capacity is safe. Keep network I/O outside database locks. Exit with proved cross-process correctness and durable compensation paths.

### P3-T01 · Make appointment creation an atomic scoped command

- [ ] **Critical** · Objective — reserve a slot and persist one appointment and its initial history together without collecting reasons for seeking treatment.
- **Cause and files** · `internal/booking/{service.go,http.go,queries.sql}`, `cmd/server/main.go`, and `000005_create_appointments.up.sql` show pool-bound queries, separate slot/appointment writes and arbitrary guest `notes`.
- **Changes and decision** · Bind booking repositories to tenant transactions. In `CreatePublicAppointment`, enforce future, available, same-therapist state with locking or a guarded update. Commit slot, appointment and history together. Stop persisting new guest treatment notes, using a compatibility response for legacy callers until P5-T11 removes the field.
- **Dependencies and regression** · P2-T02, P1-T03. Do not expose legacy note values through new use cases. Preserve timeout rules, role behavior and the active-slot unique constraint.
- **Tests, acceptance and Done** · Existing booking and PSY-A07 plus two-process contention, rollback on insert/history failure, tenant B denial, rejected past slot and a legacy request whose free text is never recorded. Done when exactly one reservation wins and failures leave no changed slot.

### P3-T02 · Commit appointment confirm, cancel and expire as conditional transitions

- [ ] **Critical** · Objective — keep appointment, slot and audit history consistent through terminal actions.
- **Cause and files** · `internal/booking/{service.go,queries.sql,booking_test.go}` sequences independent `ConfirmAdminAppointment`, `CancelAdminAppointment`, `CancelPublicAppointment` and `ExpirePendingAppointments` writes.
- **Changes and decision** · Reuse booking scope from P3-T01. Establish a stable lock order, guard allowed source states and expiry, and write history in the same transaction. Remove process-local locking only as each equivalent DB path is verified.
- **Dependencies and regression** · P3-T01. Preserve therapist ownership, repeat cancel behavior and TTL caps. Scheduled expiry is wired in P4-T07.
- **Tests, acceptance and Done** · Race confirm versus expiry and cancel versus confirm, test wrong staff/tenant, inject history failure, and confirm no released slot can still be booked by its old appointment. Done when committed combinations match the state rules and existing tests pass.

### P3-T03 · Reschedule both slots and the appointment in one transaction

- [ ] **Critical** · Objective — move an appointment without losing the original reservation on conflict or failure.
- **Cause and files** · `internal/booking/{service.go,queries.sql}`, `internal/testsupport/acceptance_phase2_test.go`; `RescheduleAdminAppointment` reads and writes two slots independently of the appointment.
- **Changes and decision** · Lock the appointment and both slot records in the agreed order. Recheck ownership, future time and availability inside the transaction, and write both states, appointment and audit history atomically.
- **Dependencies and regression** · P3-T02. Check equal-slot requests, simultaneous reschedules and reschedule-versus-cancel.
- **Tests, acceptance and Done** · Run existing PSY-A07 rescheduling cases and new two-connection conflict and mid-operation-failure cases. Done when the old slot stays reserved on every rejected move, audit history records one successful move and no deadlock survives the bounded retry policy.

### P3-T04 · Make schedule replacement and regeneration consistent

- [ ] **High** · Objective — prevent partial hours/exception edits and overlapping bookable slots.
- **Cause and files** · `internal/booking/{service.go,http.go,queries.sql}` ignores slot deletion errors and treats any slot insert error as a conflict. Migration `000005_create_appointments.up.sql` guards equal starts.
- **Changes and decision** · Deliver working-hours replacement first, exception changes second, and overlap-safe bounded regeneration third. Coordinate each edit with reservation locks and existing booked slots. Return actual DB failures. Add a new invariant migration only after checking existing overlaps and obtaining approval.
- **Dependencies and regression** · P3-T03. Preserve Tehran calendar, active reservations, therapist ownership and idempotent generation.
- **Tests, acceptance and Done** · Run booking suites. Verify replace rollback, exception create/remove, partially overlapping time ranges, repeated generation, and existing-booked-slot preservation. Done after per-slice evidence and a migrated DB test.

### P3-T05 · Make workshop reservation and expiry atomic

- [ ] **Critical** · Objective — ensure workshop registration never exceeds capacity and expired holds permit a new valid registration.
- **Cause and files** · `internal/workshops/{service.go,queries.sql,http.go}`, `cmd/server/main.go`, migration `000002_create_workshops_and_payments.up.sql`. `RegisterGuest` counts and inserts outside one database transaction and the active-phone index still covers untransitioned holds.
- **Changes and decision** · Use the tenant unit of work and one parent-row lock strategy under PSY-D03. Expire applicable old holds, check phone uniqueness and capacity, and write the new registration within the lock. Persist distinct lookup authorization and retry semantics.
- **Dependencies and regression** · P2-T02, P1-T03. Preserve free confirmation, paid 15-minute cap, registration deadline and duplicate behavior.
- **Tests, acceptance and Done** · Run PSY-A02/A03 and add two-process contention across different phones, retry after elapsed hold, same phone, and another-tenant denial. Done when durable counts stay within capacity across replicas.

### P3-T06 · Serialize workshop capacity, mutation and cancellation rules

- [ ] **Critical** · Objective — keep admin mutations and checkout callbacks consistent with active seats.
- **Cause and files** · `internal/workshops/{service.go,queries.sql,workshops_test.go}`. `UpdateWorkshopCapacity`, `UpdateWorkshop`, `CancelWorkshop` and `CancelRegistrationByAdmin` do not share the required transaction boundary.
- **Changes and decision** · Apply parent-first locking to capacity and workshop status changes first, then registration cancellation. Validate restrictions on edits after confirmed registration. Expose transaction-bound workflow operations to payment orchestration. Preserve the public service API where compatible.
- **Dependencies and regression** · P3-T05. Check concurrent admin edits versus holds and avoid premature registration cancellation or refund state changes.
- **Tests, acceptance and Done** · Capacity reduction under active seats fails without writes. Race parent cancellation and registration, verify authorized role action and rollback. Done after both slices pass existing admin and migrated concurrency checks.

### P3-T07 · Correct course purchase state and scope payment persistence

- [ ] **Critical** · Objective — insert valid purchase and attempt rows under the production role.
- **Cause and files** · `internal/payments/{service.go,queries.sql,courses_checkout_test.go}`, `cmd/server/main.go`, `000002_create_workshops_and_payments.up.sql`. `InitiateCoursePayment` uses an attempt state in `purchases.status`.
- **Changes and decision** · Use the existing `pending` purchase state and keep `initiated` on the attempt. Bind payment repositories to the same tenant unit of work as course and workshop ports. Keep external calls between transactions.
- **Dependencies and regression** · P2-T02, P2-T07, P3-T06, P1-T03. Preserve integer amount, title and reference snapshots. Transition ordering remains under P3-T09 through P3-T12.
- **Tests, acceptance and Done** · Run existing course checkout tests and a migrated role-backed initial purchase. Verify accepted purchase and attempt states, price from DB, rollback and another-tenant rejection. Done when actual PostgreSQL proves both states.

### P3-T08 · Bind payment attempts to the tenant merchant

- [ ] **Critical** · Objective — select the merchant from verified tenant scope and preserve its binding through Verify and reconciliation.
- **Cause and files** · `cmd/server/main.go`, `internal/payments/{provider.go,zarinpal_provider.go,service.go}`, and PSY-D04. A global adapter holds one merchant identifier.
- **Changes and decision** · Approve an env-backed per-tenant merchant configuration and a stable non-secret binding on attempts if needed. Scope selection and callback through the same tenant and persisted attempt. Define behavior for unavailable merchants and legacy attempts before enabling checkout.
- **Dependencies and regression** · P3-T07, P2-T11 provider contract, approved merchant enrollment and legacy mapping. Rotate without breaking in-flight attempts; never guess historical ownership.
- **Tests, acceptance and Done** · Use two synthetic merchant configurations and credential-free fixtures. Verify initiation, eligibility, callback/reconciliation binding, wrong tenant and merchant change during an attempt. Done when provider selection and historical policy have evidence and approval.

### P3-T09 · Persist payment initiation before the outbound gateway call

- [ ] **Critical** · Objective — allow a safe repeat or recovery after timeout, interrupted persistence or duplicate checkout.
- **Cause and files** · `internal/payments/{service.go,queries.sql,provider.go}`, `internal/workshops/http.go`, `internal/courses/http.go`; current authority is stored only after the external call.
- **Changes and decision** · In separately verified workshop then course slices, persist a tenant-scoped local initiation and idempotency owner before I/O. Guard the provider result update; represent unknown outcomes. Use provider-supported correlation where verified. Where correlation is unavailable, retain a visible review state and require evidence before initiating a second payment.
- **Dependencies and regression** · P3-T08, P3-T06, approval if attempt schema changes. Same key on a different purchase, hold expiry and retry-after-timeout require distinct outcomes.
- **Tests, acceptance and Done** · Test duplicate starts, provider timeout, process interruption before and after response, and provider unavailable. Show one durable initiation or a reviewable uncertainty with no silent duplicate. Done with fixture fault-test evidence and recorded provider capability.

### P3-T10 · Make workshop callback converge on one outcome

- [ ] **Critical** · Objective — finalize workshop payment and reservation atomically with replay semantics.
- **Cause and files** · `internal/payments/{service.go,queries.sql}`, `internal/workshops/queries.sql`; `HandleCallbackAndVerify` can perform multiple committed effects and read stale attempt state.
- **Changes and decision** · Verify external authority with the bound merchant and stored amount outside locks, then reacquire the attempt and reservation in one transaction using the approved lock order. Derive a single terminal state. Route a genuine late verified payment to `refund_required` without taking a seat.
- **Dependencies and regression** · P3-T09, P3-T06. A repeated callback after a refund or successful confirmation must return persisted state and preserve accounting.
- **Tests, acceptance and Done** · Existing PSY-A04 and PostgreSQL tests for concurrent callbacks, expired hold, cancellation, mismatched verification and failure at each write. Done when state converges after retries and capacity remains correct.

### P3-T11 · Make course callback and entitlement activation atomic

- [ ] **Critical** · Objective — activate one entitlement for one completed purchase and classify genuine duplicate payment for refund.
- **Cause and files** · `internal/payments/{service.go,queries.sql,courses_checkout_test.go}`, `internal/courses/queries.sql`; `handleCourseCallbackAndVerify` writes entitlement, purchase and attempt independently.
- **Changes and decision** · Use guarded transitions and the active-entitlement unique invariant within one transaction. Recheck archive state under the relevant lock. Return the stored result on replay.
- **Dependencies and regression** · P3-T10 establishes shared payment semantics. Keep course-specific rollback and archive rules intact.
- **Tests, acceptance and Done** · Verify existing course callback cases plus simultaneous distinct purchases, duplicate callbacks for one purchase, archive during settlement and fault after entitlement activation. Done when no orphan active entitlement or unrecoverable paid purchase remains.

### P3-T12 · Record refund and revoke access as one operation

- [ ] **Critical** · Objective — make `RecordRefund` preserve correct purchase and entitlement state across failure and retry.
- **Cause and files** · `internal/payments/{service.go,queries.sql,http.go}`, `internal/courses/queries.sql`; refund create, purchase transition and entitlement revoke use separate writes, with revoke errors ignored.
- **Changes and decision** · Validate owner, original amount and reference; lock the purchase and use an idempotent guarded write. Commit refund, status and linked entitlement revoke together. Preserve PSY-D04's manually completed external refund workflow.
- **Dependencies and regression** · P3-T11. Preserve differing eligibility for workshop and course refunds, reference uniqueness, and access for other purchases.
- **Tests, acceptance and Done** · Run existing refund tests and add concurrent duplicate reference, incorrect amount, wrong tenant, and rollback after failed revocation. Done when one durable refund and the exact linked revocation are recorded together.

### P3-T13 · Provide an entitlement-scoped course and lesson read model

- [ ] **High** · Objective — let legitimate purchasers load the syllabus of an archived course through an authorized route.
- **Cause and files** · `internal/courses/{http.go,buyer_service.go,queries.sql}`; existing `/v1/account/courses` lists entitlements and private stream routes sign media, while the public detail handler excludes archived content.
- **Changes and decision** · Add an approved account-detail operation by course identity, model accessible lessons under the same ownership rules as private streams, and document it in existing OpenAPI. Keep the public archive and checkout gates.
- **Dependencies and regression** · P2-T07, P3-T11, P3-T12. Protect revoked entitlements, course-vs-entitlement IDs and private media. P5-T10 owns viewer integration.
- **Tests, acceptance and Done** · Test buyer access to archived details, public absence, revoked/missing purchase, different tenant, and required no-store headers. Done when executable and contract fixtures cover the private read.

**Phase verification** · Existing Go suites, focused uncached integration with two independent processes, DB constraint and rollback evidence, synthetic provider fault tests, and prospective rows checked against real migrations. Keep buyer/admin access gates active during failure tests.

## Phase 4 · Make deployment, media and background work recoverable

**Objectives and problems** · Make release setup explicit, object-store promotion recoverable, personal-data retention reliable and disaster recovery verifiable. Address A4, C7, R5–R7 and the expiry gaps in C3.

**Sequence and outcome** · Use Phase 2 roles and Phase 3 state transitions before scheduling workers. Recovery and media slices may proceed independently after their dependencies pass. New schema migrations share one integration sequence. Exit with a reproducible deployment and evidence that interrupts do not leave irrecoverable work.

### P4-T01 · Gate runtime startup on a completed migration release step

- [ ] **High** · Objective — make fresh and upgraded Compose stacks boot under separated roles.
- **Cause and files** · `infrastructure/psychology/{docker-compose.yml,initdb/01-roles.sql}`, `apps/psychology-api/Dockerfile`, `cmd/server/main.go`, `internal/platform/deployment/deployment_test.go` start after DB connectivity while the schema is still an external assumption.
- **Changes and decision** · Establish the approved migration runner and version check, reuse the fixture mechanism where appropriate, and run migrations with a release identity. Keep API and worker connections on their own credential paths. Document warm-volume and rollback procedures.
- **Dependencies and regression** · P1-T02, P2-T01, P2-T11. Confirm migration ordering and deployed role bootstrap behavior. Coordinate final composition changes exclusively.
- **Tests, acceptance and Done** · Validate an empty database, an upgrade with representative existing rows, migration failure, and runtime role denial of schema changes. Done when readiness follows successful migration and logs contain version and roles without secrets.

### P4-T02 · Validate actual media type and enforce bounded ingress

- [ ] **High** · Objective — promote only acceptable, bounded objects from staging to verified media.
- **Cause and files** · `internal/media/{storage.go,service.go,http.go,media_test.go}`. `PresignPutURL` does not bind declared metadata and `FinalizeUpload` trusts reported type.
- **Changes and decision** · Verify ParsPack's enforceable storage upload policy in the approved test environment. Choose a supported ingress bound and check actual size plus bounded format evidence before promotion. Keep a staged rollout if existing files require classification. An unsupported upload policy blocks switching to the production upload path until an approved bounded alternative exists.
- **Dependencies and regression** · P2-T06, adapter/capability decision. Preserve supported image, audio and video formats and private delivery.
- **Tests, acceptance and Done** · Existing media tests plus innocuous inconsistent metadata, oversize rejection, unsupported containers, valid formats, and another tenant's staging ID. Done when matched S3-compatible behavior and persistence of observed metadata are verified.

### P4-T03 · Reserve media quota atomically

- [ ] **High** · Objective — cap committed assets and active reservations even under simultaneous uploads.
- **Cause and files** · `internal/media/{service.go,queries.sql,media_test.go}`, `000004_create_media.up.sql`; `GetTotalMediaUsageBytes` and the reservation insert run separately.
- **Changes and decision** · Serialize each tenant's quota decision with a scoped database guard. Account for expired reservations, finalize release and clear retry outcomes. Keep quota totals in bytes and implement any new invariant with an approved migration.
- **Dependencies and regression** · P2-T06. Coordinate with P4-T02 before touching finalization. Preserve existing quota accounting for verified, rejected and staged objects.
- **Tests, acceptance and Done** · Run real-role simultaneous reservations around the quota boundary, expiration and independent tenant quotas. Done when successful reservations plus assets never exceed the limit and denied requests leave no reserved usage.

### P4-T04 · Make finalization and staging cleanup recoverable

- [ ] **High** · Objective — turn repeated or interrupted promotion into one stable asset.
- **Cause and files** · `internal/media/{service.go,storage.go,queries.sql,media_test.go}`; `FinalizeUpload` creates a random destination and removes staging before asset persistence is durable.
- **Changes and decision** · Associate a stable final identity with each authorized reservation. Guard claims and persist metadata plus DB completion atomically. Run copy and deletion outside row locks, and make subsequent cleanup retryable. Make a repeat finalization return the existing authorized result or its persisted failure state.
- **Dependencies and regression** · P4-T02, P4-T03. Preserve quota and public/private object prefixes. Check handling of expired staging URLs and provider failures.
- **Tests, acceptance and Done** · Test duplicate finalization, copy failure, DB failure after copy, restart before staging deletion, and concurrent cleanup. Done when retries yield one verified asset and every remaining staging/final object has an auditable recovery path.

### P4-T05 · Claim outbox work durably and acknowledge provider outcomes truthfully

- [ ] **High** · Objective — guarantee bounded retry and ownership across workers.
- **Cause and files** · `internal/platform/cache/{outbox.go,worker.go,cache_test.go}`, `sqlc.yaml`, migrations `000007` and `000010`. `FetchPendingEvents` loses its lock before `Purge`, and missing credentials return success.
- **Changes and decision** · Use parameterized sqlc and an atomic claim with a lease or equivalent ownership. Scope completion/failure to the current claim; respect cooldown and maximum retries. Preserve pending events when CDN processing is disabled for launch, and report age, claimed work and failures.
- **Dependencies and regression** · P1-T02, P2-T11. Any additional claim fields and worker grants require approval. Check duplicate harmless purge delivery, restart, stale claim and least privilege.
- **Tests, acceptance and Done** · Run existing outbox tests plus two-worker DB claim, crash recovery, provider failure, missing config and claim-ownership checks. Done when real producer messages are claimable without silent success and the worker cannot mutate tenant data.

### P4-T06 · Apply approved retention without collisions or silent failure

- [ ] **High** · Objective — enforce data minimization and expiry while retaining required financial records.
- **Cause and files** · `internal/platform/retention/{cleaner.go,cleaner_test.go}`, `cmd/server/main.go`, migrations `000002` and `000005`. The cleaner uses a raw pool, overwrites active phone values with the same empty string, and ignores errors.
- **Changes and decision** · Run tenant-scoped bounded batches using authorized tenant discovery. Use sqlc, fix anonymization/index interaction with an approved mechanism, and treat legacy notes and cancellation/history data under the agreed personal-data policy. Keep disputed transactions eligible for the approved hold. Record error and backlog without retaining identifiers in logs.
- **Dependencies and regression** · P2-T02, P3-T01, legal/product retention decision for legacy data where required. Preserve uniqueness and financial snapshots.
- **Tests, acceptance and Done** · Check two old attendees in one workshop, an open financial dispute, recent appointments, pre-existing notes, replay and role denial. Done when the job makes expected changes with runtime credentials and reports any blocked batch.

### P4-T07 · Schedule expiry, reconciliation and staging cleanup through one supervised loop

- [ ] **High** · Objective — make appointment expiry, booking hold cleanup, stale payments and upload cleanup run in deployed composition.
- **Cause and files** · `cmd/server/main.go`, `internal/booking/service.go` (`ExpirePendingAppointments`), `internal/payments/service.go` (`ReconcileStalePayments`), `internal/media/service.go` (`PurgeExpiredUploads`) define jobs without production scheduling.
- **Changes and decision** · Wire independent bounded job runners in small per-job slices. Use explicit tenant/worker privileges, deadlines, cancellation and recoverable retry outcomes. Preserve a manual observation and re-run path for stuck batches.
- **Dependencies and regression** · P4-T01, P3-T02, P3-T05, P3-T12, P4-T04 through P4-T06. Check startup, shutdown and duplicate workers. Avoid treating a long task as successful on process exit.
- **Tests, acceptance and Done** · Verify real executable scheduling with controlled clock or intervals, DB access, cancellation, retry and tenant isolation. Done when unattended jobs recover seeded stale state and make failures observable.

### P4-T08 · Make backup and restore demonstrate recoverability

- [ ] **High** · Objective — fail incomplete backups or restores and prove a recovered database is usable under its runtime roles.
- **Cause and files** · `infrastructure/psychology/scripts/{backup,restore}.sh`, Compose backup service and `internal/platform/deployment/deployment_test.go` check nonempty dumps and existing table flags.
- **Changes and decision** · Fail on restore errors. Verify artifact and schema manifest, expected grants and RLS policies, and role behavior. Describe encrypted independent retention and approved credential handling, plus restore scope.
- **Dependencies and regression** · P4-T01, P2-T01. Requires an approved non-production restore environment and retention venue. Prevent a restore drill from connecting to an active deployment.
- **Tests, acceptance and Done** · Restore a fresh disposable instance from backup, fail a truncated artifact and a missing-table manifest, and verify isolation under restored roles. Done when the drill records recovery time and actual latest recoverable data point against an agreed objective.

**Phase verification** · Fresh and upgraded stack, media validation and object recovery fixtures, real-role two-worker claims, retention and expiry with bounded batches, and a measured isolated restore. The deployment scripts' existing structure tests remain part of the checks.

## Phase 5 · Make application routes report authoritative outcomes

**Objectives and problems** · Make admin and storefront interactions use one trusted request path and display actual backend state. Address C5, C6, U1–U3, R3/R4 and N4.

**Sequence and outcome** · Begin with the transport boundaries. Move feature pages individually onto the approved transport. Connect UI to payment or appointment transitions after backend failure and concurrency gates pass. Exit with accurate positive, denied, empty, conflict and unavailable states on real browser flows.

### P5-T01 · Centralize admin request context and secure settings mutations

- [ ] **High** · Objective — make admin SSR and its Astro mutation boundary use the same verified session and active clinic.
- **Cause and files** · `apps/psychology-admin/src/{middleware.ts,layouts/AdminLayout.astro,pages/settings.astro,pages/api/admin/site.ts,pages/api/admin/appearance.ts}` mix synthetic owner defaults, cookies, runtime and build env and different upstream errors.
- **Changes and decision** · Use validated server context and same-origin request checks before forwarding. Require affirmative permission for appearance controls. Choose one timeout and response mapping model and use runtime config in server-only requests. Send the clinic selector onward only from verified membership. Remove sample identity and mock settings fallback. Sequence middleware/transport first, then each settings route.
- **Dependencies and regression** · P2-T03, P2-T08, P2-T11, P2-T12, P1-T04. Preserve browser redirects and valid multi-clinic selection while maintaining Go as final authorization authority.
- **Tests, acceptance and Done** · Browser two-clinic tests, unauthenticated and revoked sessions, upstream outage, mutation origin enforcement, and denied appearance access. Done when settings never shows editable sample data and every successful mutation reaches the approved tenant.

### P5-T02 · Repair workshop management requests

- [ ] **High** · Objective — make workshop panel actions use verified clinic context and authoritative transitions.
- **Cause and files** · `apps/psychology-admin/src/pages/workshops/{index.astro,[id].astro,new.astro}`; several writes bypass settings-style authenticated transport.
- **Changes and decision** · Migrate create/edit and publish/close/cancel/attendee actions to the approved P5-T01 path. Show real status and preserve form input on errors. Avoid adding authorization policy to per-page scripts.
- **Dependencies and regression** · P5-T01, P3-T06. Check capacity/paid conflicts, receptionist versus owner roles and stale form state.
- **Tests, acceptance and Done** · Extend panel browser fixtures with two-clinic switching, owner actions, forbidden role, capacity conflict and upstream timeout. Done when actions change the verified workshop once and UI status follows persisted evidence.

### P5-T03 · Repair blog editor and publication requests

- [ ] **High** · Objective — share one authorized blog transport and consistent editor submit state.
- **Cause and files** · `apps/psychology-admin/src/pages/blog/{index.astro,[id].astro,new.astro}` duplicate direct requests and editor upload flows.
- **Changes and decision** · Migrate index actions first, then edit/create submissions and media attachment use. Keep editor data local to the feature, retain input on errors, and only attach a verified permitted media asset. Preserve publication semantics.
- **Dependencies and regression** · P5-T01, P2-T04. Media attachment verification also waits for P4-T04. Coordinate with P5-T05's upload contract.
- **Tests, acceptance and Done** · Browser create, render draft, publish, unpublish and delete in clinic A, clinic B isolation, illegal media reference and failed upload. Done after UI and mounted API results agree across the three routes.

### P5-T04 · Repair appointment and payment admin requests and dashboard data

- [ ] **High** · Objective — make appointment actions, refund actions and financial metrics authoritative.
- **Cause and files** · `apps/psychology-admin/src/pages/{appointments/index.astro,payments/index.astro,index.astro}` use distinct SSR/action transports and retain sample results after failure.
- **Changes and decision** · Convert appointment reads and actions, refund confirmation and revalidation, and dashboard metrics in three small slices. Show denied, no data and upstream unavailable states. Preserve server-confirmed roles and stable error messages.
- **Dependencies and regression** · P5-T01, P2-T05, P3-T03, P3-T12. Keep sensitive records private and preserve filters and completed refunds after refresh.
- **Tests, acceptance and Done** · Browser checks for role-restricted appointments, successful reschedule, conflict, refund, duplicate submit and unavailable metrics. Done when no fabricated totals or success appears and protected actions target the selected clinic only.

### P5-T05 · Repair course administration and media uploads

- [ ] **High** · Objective — make course/lesson editing and uploading use the same authenticated runtime transport and media state.
- **Cause and files** · `apps/psychology-admin/src/pages/courses/{index.astro,new.astro,[id]/edit.astro,[id]/lessons.astro}` and `pages/media/index.astro` use inconsistent runtime config, direct browser calls and sample data.
- **Changes and decision** · Convert course list/editor, lesson actions, and media library/uploads in three slices. Check staging PUT success before finalization and use returned identifiers from the approved tenant scope. Keep binaries on the supported storage path. Remove sample state from real routes.
- **Dependencies and regression** · P5-T01, P2-T07, P4-T04. Preserve archived listing semantics, MIME acceptance and SSR behavior with only runtime API config.
- **Tests, acceptance and Done** · Browser create/update/archive, lesson media attach, rejected upload, finalize retry and wrong-clinic isolation. Done when persisted results match the UI for all slices.

### P5-T06 · Load real home and preview content through one host-scoped loader

- [ ] **High** · Objective — show actual published records and truthful empty-surface states on `/` and `/preview`.
- **Cause and files** · `apps/psychology-site/src/{pages/index.astro,pages/preview.astro,components/HomePageContent.astro,lib/site.ts,middleware.ts}` omit data inputs, causing live routes to render demos.
- **Changes and decision** · Populate view models from tenant-published courses, workshops and posts. Distinguish an empty response from failed upstream data, and reuse the loader without overriding the preview's validated appearance parameters. Do not invent clinic biographies, credentials, categories or imagery.
- **Dependencies and regression** · P2-T04, P2-T07, P2-T12, P3-T05. Coordinate course cover rules with P5-T07/P6-T01 and preserve no-store/noindex for preview.
- **Tests, acceptance and Done** · Built SSR and browser checks with two clinics, one empty collection, no image, upstream failure, preview override and real CTA destinations. Done when fixtures live only in tests and actual route inputs are visible in test evidence.

### P5-T07 · Remove production sample responses from catalog and booking loaders

- [ ] **High** · Objective — render only available tenant data for course, workshop, article and therapist discovery.
- **Cause and files** · `apps/psychology-site/src/pages/{courses,workshops,blog,booking}`; catalog lists and detail routes initialize from samples, and course UI reads a cover URL Go does not supply.
- **Changes and decision** · Work in one listing/detail pair at a time, then handle the booking loader. Preserve correct 404 versus upstream-unavailable status. Resolve covers through a validated public-media read with a missing-image state. Keep tests using synthetic adapter responses. Do not change catalog taxonomy or product scope as part of this task.
- **Dependencies and regression** · P5-T06, P2-T06, P2-T07, P3-T05, P6-T01 for any public payload change. Avoid exposing private covers and preserve draft/archived restrictions.
- **Tests, acceptance and Done** · Add per-pair route fixtures for distinct hosts, real and empty lists, invalid slug/ID, 502/503, failed asset, draft/archived behavior and cache headers. Done when no production directory route uses sample data and all slices have evidence.

### P5-T08 · Authenticate workshop tracking and payment return

- [ ] **High** · Objective — make workshop callback and status views show verified registration/payment state.
- **Cause and files** · `internal/{payments,workshops}/http.go`, `apps/psychology-site/src/pages/workshops/{[id].astro,registrations/[id].astro}`; the return loses its authorization context and the page trusts status parameters.
- **Changes and decision** · In API then UI slices, establish a first-party scoped continuation bound to workshop registration and browser, keep `private, no-store`, and remove query status as a source of truth. Remove successful fallbacks on API errors. Use a safe resume outcome for uncertain initiation and keep reconciliation authoritative.
- **Dependencies and regression** · P3-T09, P3-T10, P2-T12, P1-T04. Preserve free registration, the valid gateway round trip and PSY-D02 assisted recovery for another device.
- **Tests, acceptance and Done** · Browser free and paid flows, canceled/failed/unavailable provider, reload, missing authorization and another tenant. Done when status is read from authenticated state and credential redaction is verified on the exercised paths.

### P5-T09 · Authenticate appointment tracking and remove fabricated statuses

- [ ] **High** · Objective — make appointment tracking reflect only valid backend responses.
- **Cause and files** · `internal/booking/http.go`, `apps/psychology-site/src/pages/appointments/[id].astro`, `src/components/BookingFlow.tsx`; private credentials travel in URLs and timeout can leave a synthetic pending appointment.
- **Changes and decision** · Adapt the approved first-party tracking continuation pattern with separate appointment scope. Preserve valid status, expiry and cancellation, deny incorrect or missing credential, and return explicit temporary failure. Redact historical URL credential forms during any documented compatibility window.
- **Dependencies and regression** · P3-T02, P2-T12, P5-T08 continuation policy. Coordinate `BookingFlow.tsx` with P5-T11 and keep authorized old tracking links only under approved policy.
- **Tests, acceptance and Done** · Browser begin, reload, cancel, timeout, missing or expired access and second device/tenant cases. Done when no URL-driven fixture is shown and current role and no-store checks pass.

### P5-T10 · Use authorized course data in the private viewer

- [ ] **High** · Objective — open entitled archived content and keep media state consistent with selected lesson.
- **Cause and files** · `apps/psychology-site/src/pages/account/courses/{index.astro,[id].astro}` initializes the viewer through public course detail and applies asynchronous stream results directly.
- **Changes and decision** · Call the entitlement-scoped detail operation. Guard stream requests by selected lesson, clear outdated sources at transitions and report expiration/revocation. Preserve the private signing path and manual or automatic renewal consistent with the existing contract.
- **Dependencies and regression** · P3-T13, P2-T10, P1-T04. Preserve published video/audio playback, range/seek and logout behavior.
- **Tests, acceptance and Done** · Browser existing archived buyer, unentitled buyer, revoked access, quick lesson switching and renewal. Done when only the selected authorized lesson can play and the viewer explains every rejected state.

### P5-T11 · Make booking state request-scoped

- [ ] **High** · Objective — keep the displayed slots, selected date, therapist and submit result consistent.
- **Cause and files** · `apps/psychology-site/src/components/BookingFlow.tsx`, `pages/booking/index.astro`, booking SSR tests. Older fetches can replace the current list, and zero matches in day mode exposes a different day's slots.
- **Changes and decision** · Bind request results to current provider/date, cancel or ignore superseded requests, and keep filter mode explicit. On changing input, invalidate any incompatible picked slot. Remove the guest reason field and use server error/confirmation states. Separate feature state from presentational controls where it simplifies changes.
- **Dependencies and regression** · P5-T07 loader slice, P5-T09, P3-T01 through P3-T04. Preserve ISO contract, keyboard focus and normal booking updates. Date helper changes land separately at P6-T04.
- **Tests, acceptance and Done** · Browser intentionally reversed benign fixture responses, empty selected day, explicit all-days toggle, conflict refresh, invalid/pending result and keyboard form submission. Done when obsolete results cannot update state and no reason for care is sent.

**Phase verification** · Fresh production-mode SSR assets, tenant A and B browser and API traces, accessible error behavior, and verified persisted outcomes. Use the existing Node route suites for HTML regression and browser evidence for interactive acceptance.

## Phase 6 · Stabilize API contracts, templates and accessibility

**Objectives and problems** · Establish verified HTTP shapes, coherent shell and token ownership, accessible controls and stable appearance updates. Address S2, C8, U4–U7, D3/D4 and N1/N4.

**Sequence and outcome** · The transport contract task can start after its Phase 2 prerequisites to unblock Phase 5 callers. UI implementation follows the stable route behavior it renders. Coordinate the existing PSY-7.7 track at each shared file boundary. Exit with matching actual HTTP contracts, typed UI boundaries and computed browser evidence.

### P6-T01 · Match API contract, transport and mounted routes

- [ ] **Medium** · Objective — reconcile public blog shape, the buyer profile route, media representation and success/error parsing.
- **Cause and files** · `packages/contracts/psychology/openapi.yaml`, `packages/psychology-api-client/src/{index,types}.ts`, `internal/{blog,courses}/http.go` and their consumers carry independently maintained definitions.
- **Changes and decision** · First inventory consumers and confirm the existing array shape and `/account/auth/me` before choosing compatibility or versioning. Update one endpoint family at a time. Support bounded pagination and cover/reference handling only after agreeing the contract. Handle bodyless success safely without changing current JSON routes. Prefer checked-in API parity tests before introducing any generator dependency.
- **Dependencies and regression** · P1-T03, P2-T04, P2-T07. P3-T13 already documents the new private read. P5-T07 uses the finalized public payload rules. Preserve clients that consume raw arrays.
- **Tests, acceptance and Done** · Capture real mounted 2xx/4xx shapes, blog list, buyer profile, course cover handling and JSON/empty response helper cases. Done when OpenAPI, server, checked-in client and applicable SSR fixtures agree.

### P6-T02 · Make the admin shell responsive and navigable

- [ ] **Medium** · Objective — support a predictable shell at the supported widths without hiding workflow actions.
- **Cause and files** · `apps/psychology-admin/src/layouts/AdminLayout.astro`, `apps/psychology-admin/test/admin-pages.test.mjs`. The fixed sidebar and content layout compete for width.
- **Changes and decision** · Let the root shell own nav breakpoints, current-route semantics, resize behavior and workspace region. Reuse existing primitives and native controls where suitable. Keep per-page content layout in the feature.
- **Dependencies and regression** · P5-T01, P1-T04. Check focus order, nav reveal/dismiss, logout and RTL.
- **Tests, acceptance and Done** · Capture built desktop light navigation and the project's narrow-layout acceptance fixture, including keyboard focus and horizontal overflow. Done when valid navigation, heading and logout are reachable at both sizes.

### P6-T03 · Correct category navigation and associate shared field feedback

- [ ] **Medium** · Objective — give catalog controls real destinations and connect errors to their inputs.
- **Cause and files** · `apps/psychology-site/src/pages/{courses,blog,workshops}/index.astro`, `packages/ui/src/interactive/{Select,PersianDatePicker}.tsx`. Fragment chips lack destinations, and help/error spans lack ownership.
- **Changes and decision** · Deliver three small slices: replace invalid chips with genuine links or real filters after taxonomy approval, wire Select feedback, then wire date-picker feedback. Avoid tab semantics for navigation links. Coordinate filters with PSY-7.7.5 and PSY-7.7.6.
- **Dependencies and regression** · P5-T07 for catalog pages, P1-T04. Widget slices can proceed independently. Keep a visible empty state and keyboard usable focus behavior.
- **Tests, acceptance and Done** · Check links or working filter results, tab order, accessible names and descriptions, invalid state announcement and RTL interaction. Done when no visible inert control remains and help/error text is programmatically associated.

### P6-T04 · Make civil date formatting timezone-independent

- [ ] **Medium** · Objective — preserve the intended day from UI field to ISO contract.
- **Cause and files** · `packages/ui/src/interactive/{date-utils.ts,PersianDatePicker.tsx}`, `apps/psychology-site/src/components/BookingFlow.tsx`. A civil date is formatted as an instant in the host timezone.
- **Changes and decision** · Use calendar values for dates and an explicit clinic timezone for timestamps. Preserve existing ISO-to-Jalali semantics and clarify invalid input handling. Migrate the booking formatter after the utility regression is green.
- **Dependencies and regression** · P5-T11 for booking consumer edits. Utility-only work can start sooner. Preserve month/year edges, Tehran timestamps and form field serialization.
- **Tests, acceptance and Done** · Run `pnpm --filter @psychology/ui test` and add timezone-matrix checks covering UTC, Tehran and a west-of-UTC timezone plus year-boundary and ISO round-trip cases. Done when one civil date renders identically across runtime timezones.

### P6-T05 · Put shell spacing and card semantics under explicit ownership

- [ ] **Medium** · Objective — remove double gutters and align route, shell and card responsibilities.
- **Cause and files** · `apps/psychology-site/src/layouts/BaseLayout.astro`, `src/templates/{contract.ts,view-models.ts,editorial/Shell.astro}`, `src/components/HomePageContent.astro` and the four card families.
- **Changes and decision** · First define the viewport/container responsibility with PSY-7.7.3. Assign one gutter owner. Then migrate one card family at a time to typed view models with authorized URLs and common markup where it is stable. Keep genuine shell layout and navigation differences.
- **Dependencies and regression** · P5-T06, P5-T07, P6-T06 for coordinated Editorial visuals. Avoid simultaneous changes to `BaseLayout` and keep rendered `h1`, landmarks, RTL and all four template routes stable.
- **Tests, acceptance and Done** · Compare documented viewport gutter and content alignment, and check all template variants, CTA destinations and empty-media layout. Done when each migrated family passes and visual evidence identifies the shell spacing owner.

### P6-T06 · Give the selected palette ownership of computed color

- [ ] **Medium** · Objective — make a palette change observable in the same template and preserve readable foreground/background pairs.
- **Cause and files** · `packages/ui/src/{palette-catalog.ts,styles/palettes.css}`, `apps/psychology-site/src/templates/editorial/theme.css`, `internal/settings/service.go`, `000011_add_palette_id_to_site_settings.up.sql`. Editorial competes with palette rules for the same properties.
- **Changes and decision** · Fix CSS ownership first. Coordinate the independent Stitch palette with PSY-7.7.2, using its documented tokens and a new approved migration if the palette constraint changes. Verify foreground contrast against actual surfaces and keep palette validation synchronized. Avoid simultaneously rewriting shell layout or version semantics.
- **Dependencies and regression** · P1-T04, P2-T04 and P5-T01 for end-to-end publication proof, PSY-7.7.1 baseline for exact reference matching. Preserve current template presets and invalid-palette fallback.
- **Tests, acceptance and Done** · Compare computed colors for two palettes on Editorial, one other template, and two Hosts; verify preview/save, migration round trip where applicable, and representative text, badges and focus. Done when switching palette changes intended colors and both PSY-A14 and applicable PSY-A15 assertions pass.

### P6-T07 · Define version behavior and unify theme metadata

- [ ] **Medium** · Objective — make persisted template version and supported metadata descriptive and verifiable.
- **Cause and files** · `packages/ui/src/{template-catalog,palette-catalog}.ts`, `apps/psychology-site/src/templates/registry.ts`, `internal/settings/{service.go,settings_test.go}` repeat supported values, while SSR selects only by template ID.
- **Changes and decision** · Decide whether version guarantees render compatibility or tracks a release marker. Keep current values during that decision. Introduce one approved manifest or explicit mechanical parity checks and add the matching resolution behavior. Sequence validation, SSR and docs separately.
- **Dependencies and regression** · P6-T06, decision recorded under PSY-D09/D10. Preserve supported existing pairs, legacy fallbacks and preview parity.
- **Tests, acceptance and Done** · Verify existing and unknown ID/version pairs, upgrade or fallback behavior, and parity across CSS, Go and Astro. Done when version behavior is documented and verifiable by the guard suite.

### P6-T08 · Preserve omitted appearance fields atomically

- [ ] **Medium** · Objective — make partial appearance writes safe under concurrency.
- **Cause and files** · `internal/settings/{service.go,queries.sql,isolation_test.go}`; `UpdateAppearance` reads current radius/scale then writes them back.
- **Changes and decision** · Preserve omitted fields in the scoped SQL write or lock the row while merging; make initial defaults explicit. Keep profile fields separately owned and insert the publication event in the same commit.
- **Dependencies and regression** · P2-T02, P4-T05 outbox contract. Serialize settings queries with P6-T07/P7-T04. Preserve defaulting and operation-specific permissions.
- **Tests, acceptance and Done** · Run live two-connection disjoint updates, first write, content/appearance overlap and DB rollback. Done when updates preserve fields they omit and existing isolation passes.

### P6-T09 · Decide the post-pilot appearance authorization policy

- [ ] **Medium** · Objective — record whether appearance remains pilot-operated or becomes an explicit tenant-scoped capability.
- **Cause and files** · `internal/settings/http.go`, `docs/psychology/modules/site-appearance.md`, `docs/psychology/DECISIONS.md`. A configured account currently represents the feature's authorization policy.
- **Changes and decision** · Record the owner decision and its security model. If retained, verify revocation/recovery and keep the operational configuration explicit. If self-service is approved, schedule a separate least-privilege grant, authorized migration if needed, audit and rollout slice under the signed decision.
- **Dependencies and regression** · P2-T03, P5-T01, owner approval. Preserve pilot access and restrictions until approved rollout evidence exists.
- **Tests, acceptance and Done** · Specify allow and deny cases for the selected model and verify the effective deployed route policy in fixture mode. Done when the decision, owner, implementation follow-up if applicable and regression evidence are recorded.

**Phase verification** · Mounted API shape checks, package tests, built-site browser checks for computed palette and layout, desktop light interaction checks, matching RTL acceptance cases, and two-host published/preview verification. Preserve existing PSY-A14 and PSY-A15 scope.

## Phase 7 · Bound load and qualify the production release

**Objectives and problems** · Reduce verified request and query amplification, make cache safety measurable, and close target-environment questions. Address P1–P5, R8, plus production evidence gaps in D1 and N2.

**Sequence and outcome** · Start measurements after correctness has stabilized. Keep the initial SSR release independent from shared HTML activation. Gate publication and playback changes before any cache experiment. Exit with an evidence-backed release decision, capacity baselines and a separate cache activation decision.

### P7-T01 · Eliminate duplicate site resolution and bound sitemap work

- [ ] **Medium** · Objective — resolve tenant/site once and give SSR and sitemap a bounded deadline.
- **Cause and files** · `apps/psychology-site/src/{middleware.ts,lib/site.ts,pages/sitemap.xml.ts}` and public detail/list routes duplicate profile requests and use different timeout and partial-response behavior.
- **Changes and decision** · Migrate one route family per slice to the request-local context. Use bounded independent sitemap reads and an explicit availability/error policy before writing XML. Keep verified host and valid XML URL construction.
- **Dependencies and regression** · P2-T12, P5-T06, P5-T07. Coordinate API client deadline semantics after P6-T01. Preserve generated paths, canonical host, and genuine empty sitemap behavior.
- **Tests, acceptance and Done** · Assert one site-resolution call, bounded upstream failure and same-tenant published URLs only. Record request counts and latency on a fixed fixture. Done when request counts and timeout behavior match the documented route contract.

### P7-T02 · Bound workshop reads, metrics cost and rate-limit memory

- [ ] **Medium** · Objective — avoid per-record capacity queries, observe report cost, and bound booking bookkeeping.
- **Cause and files** · `internal/workshops/{service.go,queries.sql}`, `internal/reports/{service.go,queries.sql}`, `internal/booking/ratelimit.go` have N+1 reads, aggregate scans and retained client keys.
- **Changes and decision** · Deliver separately: aggregate capacity for one page of workshops, measure and bound report queries without changing snapshot semantics, then bound rate-limit state and test idle cleanup. Add indexes only when the query plan shows a need and an approved migration is available.
- **Dependencies and regression** · P2-T05, P3-T03, P3-T06. Preserve counts, sorting, empty page behavior and tenant-wide admission expectations.
- **Tests, acceptance and Done** · Compare query counts and plans at representative fixture sizes, verify capacity totals and metrics, and demonstrate bounded client-key storage with controlled time. Done when each slice has measurements and stable business results.

### P7-T03 · Set route-specific JavaScript and interaction budgets

- [ ] **Medium** · Objective — measure and, where warranted, reduce the cost of booking and course interactions.
- **Cause and files** · `apps/psychology-site/src/components/BookingFlow.tsx`, `pages/booking/index.astro`, course/workshop detail routes and `test/acceptance-a14.test.mjs`. Build chunks and SSR regexes do not describe actual browser downloads.
- **Changes and decision** · Measure transferred script bytes and interaction behavior in the built browser fixture. Set explicit budgets with the project owner and implement targeted loading boundaries if needed. Keep content routes SSR and preserve interactions that require JavaScript. Record before/after and test conditions.
- **Dependencies and regression** · P5-T10, P5-T11, P6-T04, P1-T04. Verify hydration, calendar and active checkout behavior after bundle changes.
- **Tests, acceptance and Done** · Record route waterfall, script count, cold and warm transfers, and a representative booking interaction. Done when measured routes satisfy the agreed budgets and required workflows remain usable.

### P7-T04 · Produce complete invalidation events and prove host isolation

- [ ] **High** · Objective — align publication events with all affected routes and verified aliases before any shared HTML cache activation.
- **Cause and files** · `internal/settings/{service.go,queries.sql}`, `internal/{blog,courses,workshops}` publication paths, `internal/platform/cache/{key.go,outbox.go,cache_test.go}`. Current producer omits details and aliases and the test fabricates them.
- **Changes and decision** · Define a route-family invalidation contract, including detail, list, sitemap and home as applicable. First cover appearance, then each publication module in separate small slices. Derive hosts from verified tenant data and write events in the transaction. Preserve retry semantics from Phase 4.
- **Dependencies and regression** · P4-T05, P6-T08, P2-T04, P2-T07, P3-T06, P5-T07. Keep shared HTML cache disabled pending P7-T05 and a separate approved activation.
- **Tests, acceptance and Done** · Enqueue events from real mutations, verify rollback yields no event, and verify purge of A including aliases while B remains unaffected. Done when actual producer/worker outputs and private path bypass are observable in fixture mode.

### P7-T05 · Remove short-lived media grants from reusable article HTML

- [ ] **High** · Objective — make legitimate article playback survive stale HTML without weakening access checks.
- **Cause and files** · `apps/psychology-site/src/pages/blog/[slug].astro`, `internal/media/{http.go,service.go}`. Article HTML embeds temporary playback grants with conflicting lifetime.
- **Changes and decision** · Choose the approved on-demand refresh or stable public delivery model under PSY-D07. Keep signatures out of reusable HTML where the cache lifetime can exceed them and verify attachment authorization at the media boundary. Decide how to preserve document-only rendering and playback degradation consistent with the route budget.
- **Dependencies and regression** · P4-T02, P4-T04, P5-T07 blog slice. Coordinate with P7-T03 instead of requiring an arbitrary JS target. Preserve range playback and public/private media separation.
- **Tests, acceptance and Done** · Advance fixture time past the existing five-minute grant, exercise playback refresh and media denial through tenant B, and verify cache headers. Done when reused content does not serve an expired embedded grant and course media remains entitlement-scoped.

### P7-T06 · Qualify deployment, providers and ingress

- [ ] **High** · Objective — determine whether the approved launch environment satisfies release conditions with two tenant hosts.
- **Cause and files** · `infrastructure/psychology/{Caddyfile,docker-compose.yml}`, `docs/psychology/{SUPPORT_RUNBOOK,ACCEPTANCE,DECISIONS}.md`. Repository tests do not establish actual target TLS, origin routing, provider readiness or private cache policy.
- **Changes and decision** · On an approved non-production target, record TLS/provider capability, enabled features, DNS and host routing expectations, origin restrictions, trusted headers, redacted logs and cache bypass. Exercise synthetic checkout, storage, access delivery, recovery and monitoring under configured adapters. Obtain separate sign-off to deploy or to activate shared HTML caching later.
- **Dependencies and regression** · All other Critical and High remediation tasks in the approved release scope. Deferral requires a signed scope decision and a disabled affected route. Require P4-T08 restore evidence. Shared HTML activation also requires explicit PSY-D10 approval and the completed P7-T04/P7-T05 checks. Preserve private no-store and existing verified aliases.
- **Tests, acceptance and Done** · Document each required two-host probe and its result, a post-deploy readiness and incident rollback drill in staging, and verified provider behavior. Done when the owner can sign an evidence-backed release decision. If an enabled provider or target property is unverified, record the blocker and keep that release gate open.

**Phase verification** · Capture before/after route request counts, browser assets, DB plans and test conditions. Prove producer-generated invalidation, verify private media and ingress isolation, sign the target release checklist, and keep activation of shared HTML as its own decision.

## Coordination with the existing Stitch work

The worktree contains the registered [PSY-7.7 visual and data track](docs/psychology/TASKS.md#psy-77--تطبیق-دقیق-سایت-عمومی-با-stitch) and [PSY-A15 acceptance](docs/psychology/ACCEPTANCE.md#psy-a15--تطبیق-دقیق-با-stitch). Reuse the same implementation slice and evidence for overlapping criteria.

| Project task | Roadmap coordination |
| --- | --- |
| PSY-7.7.1 | Use P1-T04 for the fixture browser. Capture the six references at the specified desktop width with allowed images and the intended font. |
| PSY-7.7.2 / PSY-7.7.3 | P6-T06 before P6-T05. Verify computed tokens before capturing aligned shell views. Serialize edits to layouts and the palette manifest. |
| PSY-7.7.4 through PSY-7.7.6 | Reuse P5-T06, P5-T07 and P6-T01 and P6-T03 for authoritative data, covers, categories and contract parity. Confirm approved content and media sources per clinic. |
| PSY-7.7.7 | Reuse P5-T09, P5-T11 and P6-T04 for the existing in-person booking model. Online appointments or session-type pricing remain outside this track. |
| PSY-7.7.8 | Combine the two-host DB and browser evidence from Phase 5 and Phase 6 with publication/outbox evidence from P7-T04. Keep remaining visual acceptance differences explicit. |

## Open decisions and release blockers

| Decision or evidence | Owner or dependency | Unblocks |
| --- | --- | --- |
| Approved release scope and availability of payment merchant, buyer access delivery, ParsPack and Arvan test/target accounts. Buyer SMS is outside the recorded pilot scope unless the owner changes that decision. | Project owner and approved providers | P2-T11, P3-T08, P3-T09, P4-T02, P7-T06 |
| Tenant merchant binding, treatment of legacy attempts, and provider correlation after ambiguous initiation. | Payments owner and approved provider evidence | P3-T08, P3-T09 |
| Narrow cross-tenant lookup and retention role grants, plus handling of legacy notes and historical tracking links. | Project owner and DB/security reviewers | P2-T03, P4-T06, P5-T08, P5-T09 |
| Approved target ingress, certificate provisioning, credential rotation status, backup venue and recovery objectives. | Platform operator and project owner | P2-T01 where credentials are in use, P4-T08, P7-T06 |
| Browser/fixture runner, usable Stitch media and real clinic content sources. Preserve the existing project's design scope and topology. | Project owner and QA | P1-T04 browser work, corresponding PSY-7.7 slices |
| Catalog compatibility/versioning, template version semantics and future appearance delegation. | API/product owners | P6-T01, P6-T07, P6-T09 |
| Route budget targets and later shared HTML cache activation. | Project owner with measured browser and target CDN evidence | P7-T03, optional cache activation |

Unexecuted remediation tasks remain **Not started**. An unresolved gate becomes **Blocked** when its otherwise-ready task cannot progress without the named input. Maintain the existing release hold for Critical and High defects until the applicable gates have verified evidence.

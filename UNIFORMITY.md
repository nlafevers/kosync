# KOSYNC/KOPDS Uniformity Inventory

This project is being refactored alongside its sibling project so equivalent behavior has identical names and identical code wherever that is practical. Keep this inventory current when adding or changing cross-project behavior.

## Currently Identical Functions

- `runCLI`
- `printUsage`
- `passwordFromArgs`
- `readPasswordInteractively`
- `HashPassword`
- `CheckPassword`
- `logger.New`
- `OpenSQLite`
- `Migrate`
- `NewSQLite`
- `NewStorage`
- `EnforceStorageCap`
- `vacuum`
- `resolveExecutablePaths`
- `resolvePath`
- `generateRequestID`
- `IPRateLimiter`
- `NewIPRateLimiter`
- `GetLimiter`
- `clientIP`
- `RateLimitMiddleware`
- HTTP Routing (native `net/http.ServeMux`)

## Currently Identical Config Fields

- `RateLimitEnabled`
- `RateLimitPerMinute`
- `RateLimitBurst`
- `TrustProxyHeaders`

## High-Confidence Standardization Targets

- Config loading and path resolution
- CLI user-management output and error behavior

## Intentional Project Boundaries

- KOPDS owns OPDS catalog, Calibre scanner, image cache, book repository, and link-generation behavior.
- KOSYNC owns KOReader sync protocol handlers, progress storage, registration, and header authentication behavior.
- Database schemas may differ when the stored domain differs, but shared lifecycle helpers should remain identical.
- User-storage abstraction differs by design: KOPDS owns a `domain.UserRepository` (clean-architecture interface, context-based, `sqliteUserRepository`); KOSYNC owns flat `*Storage` user methods. This is a documented boundary, not drift — do not force one app onto the other's shape.
- `openCLIStorage` differs as a consequence of the boundary above: KOPDS returns `(*sql.DB, domain.UserRepository)` and injects `NewUserRepository`; KOSYNC returns `(*sql.DB, *database.Storage)` and injects `NewStorage`. The surrounding CLI flow (`runCLI`, `printUsage`, password helpers) stays identical.

## Final Audit Notes

- `pruneStorageCapRecords` intentionally differs because KOPDS prunes catalog sync-state rows while KOSYNC prunes progress rows.
- `config.Load` intentionally differs because each project has different domain settings; shared path-resolution helpers remain identical.
- KOPDS has a repository-level `EnforceStorageCap` adapter to satisfy the book repository interface; KOSYNC calls storage directly.
- `EnforceStorageCap` returns early if `capMB <= 0` at the top of the function, before any file I/O. Both apps share this guard.
- Database lifecycle is `OpenSQLite` → `Migrate` → inject (`NewBookRepository` / `NewUserRepository` on KOPDS; `NewStorage` on KOSYNC). There is no `InitDB` helper.
- `generateRequestID` allocates 16 bytes via `crypto/rand.Read`, encodes as hex, and falls back to a timestamp-based ID on failure. Identical in both apps.
- `PRAGMA foreign_keys=ON` is set during SQLite connection init alongside WAL mode in both apps.
- Rate-limit helpers (`IPRateLimiter`, `NewIPRateLimiter`, `GetLimiter`, `clientIP`, `RateLimitMiddleware`) are copy-paste identical. Config fields `RateLimitEnabled`, `RateLimitPerMinute`, `RateLimitBurst`, and `TrustProxyHeaders` use the same names and defaults.
- Similarity matches involving unrelated `Close` methods are false positives and are not uniformity targets.
- Both KOPDS and KOSYNC `create-user` CLI commands fail if the user already exists to prevent accidental overwrites. Use `change-password` to update.

## Round 3 Audit (June 2026): Pruning Candidates And Corrections

A third audit checked whether two rounds of uniformity work left behind uniform code that is never wired in, or legacy code that was never removed. It is documentation-only: the items below are recorded but not yet changed. Security was clean (the KOSYNC registration path was re-verified against account-takeover and is safe — `HandleUserCreate` checks `GetUserHash` before any write). The work is bloat removal.

### Inventory corrections

- `openCLIStorage` was previously listed under "Currently Identical Functions" but has diverged by design; it is now described under "Intentional Project Boundaries" above.
- `UpdatePassword(username, passwordHash string) error` was previously listed as identical but is not the same live function in each app: KOPDS's live update is `sqliteUserRepository.UpdatePassword` (context-based), while the textually-shared `(*Storage).UpdatePassword` is dead in KOPDS (see below) and a wrapper over `UpdateUserPassword` in KOSYNC. The entry was removed pending the prune.

### KOPDS pruning candidates (zero non-test callers unless noted)

- `(*Storage)` user methods `CreateUserIfNotExists`, `SaveUser`, `GetUserHash`, `UpdatePassword`, `DeleteUser` (`internal/database/sqlite.go`) — uniformity mirror of KOSYNC that was never wired in; production uses `sqliteUserRepository`. Remove.
- `NewStorage` (`internal/database/sqlite.go`) — production builds `&Storage{db: r.db}` directly (`book_repository.go`). Remove; keep the `Storage` struct (carries `EnforceStorageCap`).
- `UserRepository.Save` / `sqliteUserRepository.Save` (`internal/domain/interfaces.go`, `internal/database/user_repository.go`) — create uses `CreateUserIfNotExists`, update uses `UpdatePassword`. Remove.
- `(*BookService).GetLinkGenerator` (`internal/service/book_service.go`) — no callers anywhere. Remove.
- Never-populated OPDS Atom symbols (`internal/opds/atom.go`): `AtomNamespace`, the `IndirectAcquisition` type + `Link.IndirectAcquisitions` field, the `Category` type + `Entry.Categories` field. Remove. Lower priority: never-assigned scalar slots `Link.Count/Price/Currency`, `Entry.Published/Rights`, `Feed.Icon`.
- `go.mod`: `golang.org/x/time` is mislabeled `// indirect` but is a direct dependency; `go mod tidy` promotes it.

### KOSYNC pruning candidates

- `NewSQLite` (`internal/database/sqlite.go`) — no callers; production calls `OpenSQLite` directly. (Mirror image of KOPDS, which calls `NewSQLite`.) Standardize the call-site name across both apps and remove the unused wrapper.
- `GetRequestID` (`internal/api/context.go`) — no callers; the request ID is emitted through the bound logger, never read back. Remove.
- `models.User` (`internal/models/models.go`) — no references; auth/registration use raw strings. Remove (keep `models.Progress`).
- `UpdateUserPassword` (`internal/database/sqlite.go`) — reached only by the `UpdatePassword` wrapper added in UR2-3.3. Inline into `UpdatePassword` and delete the legacy name.
- `SaveUser` (`internal/database/sqlite.go`) — only caller is `CreateUser`; redundant hop, consider inlining. Adjacent create paths are inconsistently named (`CreateUser` for the API upsert vs `CreateUserIfNotExists` for the CLI guard).

### Duplicated storage-cap logic (both apps)

- `(*Storage).EnforceStorageCap` stats the file and gates on `capMB`/size, then calls the package-level `enforceStorageCap`, which re-checks `capMB <= 0`, re-stats, and re-checks size before pruning. The free function's guards are dead in practice. Collapse the free function into the method so the stat and gate run once; keep both apps identical through the change.

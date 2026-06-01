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
- KOSYNC owns the `NewStorage` constructor because its `Storage` is the primary storage object; KOPDS builds `&Storage{...}` inline as a thin storage-cap adapter and has no `NewStorage`. Do not re-add `NewStorage` to KOPDS for the sake of symmetry.
- Both apps open SQLite via `OpenSQLite` directly. There is no `NewSQLite` wrapper in either app.

## Final Audit Notes

- `pruneStorageCapRecords` intentionally differs because KOPDS prunes catalog sync-state rows while KOSYNC prunes progress rows.
- `config.Load` intentionally differs because each project has different domain settings; shared path-resolution helpers remain identical.
- KOPDS has a repository-level `EnforceStorageCap` adapter to satisfy the book repository interface; KOSYNC calls storage directly.
- `EnforceStorageCap` returns early if `capMB <= 0` at the top of the function, before any file I/O. Both apps share this guard.
- Database lifecycle is `OpenSQLite` → `Migrate` → inject (`NewBookRepository` / `NewUserRepository` on KOPDS; `NewStorage` on KOSYNC). There is no `InitDB` helper and no `NewSQLite` wrapper.
- `generateRequestID` allocates 16 bytes via `crypto/rand.Read`, encodes as hex, and falls back to a timestamp-based ID on failure. Identical in both apps.
- `PRAGMA foreign_keys=ON` is set during SQLite connection init alongside WAL mode in both apps.
- Rate-limit helpers (`IPRateLimiter`, `NewIPRateLimiter`, `GetLimiter`, `clientIP`, `RateLimitMiddleware`) are copy-paste identical. Config fields `RateLimitEnabled`, `RateLimitPerMinute`, `RateLimitBurst`, and `TrustProxyHeaders` use the same names and defaults.
- Similarity matches involving unrelated `Close` methods are false positives and are not uniformity targets.
- Both KOPDS and KOSYNC `create-user` CLI commands fail if the user already exists to prevent accidental overwrites. Use `change-password` to update.
- `UpdatePassword` is intentionally NOT cross-identical: KOPDS's live update is `sqliteUserRepository.UpdatePassword` (context-based, clean-architecture interface); KOSYNC's is `(*Storage).UpdatePassword`. This follows the standing rule "do not force domain-specific code into artificial sameness."

## Round 3 Audit (June 2026): Completed

A third audit checked whether two rounds of uniformity work left behind uniform code that was never wired in or legacy code that was never removed. Security was clean (the KOSYNC registration path was re-verified against account-takeover and is safe). Round 3 was bloat removal only, implemented in `ur3-roadmap.md`.

The following dead and redundant items were removed: the dead `*Storage` user methods (`CreateUserIfNotExists`, `SaveUser`, `GetUserHash`, `UpdatePassword`, `DeleteUser`) from KOPDS, `NewStorage` from KOPDS, `UserRepository.Save` from KOPDS, `GetLinkGenerator` from KOPDS, unused OPDS Atom symbols from KOPDS (`AtomNamespace`, `IndirectAcquisition`, `Category`, and several never-assigned scalar fields), `GetRequestID` from KOSYNC, `models.User` from KOSYNC, `UpdateUserPassword` from KOSYNC (inlined into `UpdatePassword`), `SaveUser` from KOSYNC (inlined into `CreateUser`), and `NewSQLite` from both apps. The duplicated storage-cap size gate was also collapsed so `os.Stat` runs once per `EnforceStorageCap` invocation.

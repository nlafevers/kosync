# KOSYNC/KOPDS Uniformity Inventory

This project is being refactored alongside its sibling project so equivalent behavior has identical names and identical code wherever that is practical. Keep this inventory current when adding or changing cross-project behavior.

## Currently Identical Functions

- `runCLI`
- `printUsage`
- `openCLIStorage`
- `passwordFromArgs`
- `readPasswordInteractively`
- `HashPassword`
- `CheckPassword`
- `UpdatePassword(username, passwordHash string) error`
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

# KOSYNC/KOPDS Uniformity Inventory

This project is being refactored alongside its sibling project so equivalent behavior has identical names and identical code wherever that is practical. Keep this inventory current when adding or changing cross-project behavior.

## Currently Identical Functions

- `passwordFromArgs`
- `readPasswordInteractively`

## High-Confidence Standardization Targets

- CLI usage and command dispatch: `printUsage`, `runCLI`, and user-command helpers
- Password helpers: `HashPassword` and `CheckPassword`
- Logger construction: `logger.New`
- Config loading and path resolution
- SQLite open/create/permission/WAL setup
- Storage-cap threshold handling and VACUUM flow
- CLI user-management output and error behavior

## Intentional Project Boundaries

- KOPDS owns OPDS catalog, Calibre scanner, image cache, book repository, and link-generation behavior.
- KOSYNC owns KOReader sync protocol handlers, progress storage, registration, and header authentication behavior.
- Database schemas may differ when the stored domain differs, but shared lifecycle helpers should remain identical.

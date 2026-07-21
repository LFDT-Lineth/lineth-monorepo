# WinstonLogger log-line truncation (defense-in-depth)

Date: 2026-07-21
Branch: `fix/native-yield-oversized-log-lines`

## Problem

The `native-yield-automation-service-l1` pod in prod (`linea-prod-eks`, `zk` namespace) emits a log line of ~18.6 MB once every gauge-poll cycle (`GAUGE_METRICS_POLL_INTERVAL_MS`, 10 min in prod). The line is the DEBUG-level `getPendingDeposits return value` dump from `ts-libs/linea-shared-utils/src/clients/BeaconNodeApiClient.ts`, which serializes the full pending-deposits array (~41,000 entries, each carrying a 96-byte BLS signature). Loki rejects any entry over its 256 KB (262,144 byte) max entry size, so the Grafana Alloy sidecar drops the line and logs `max entry size '262144' bytes exceeded ... length '18659278' bytes` (~282 drops over 2 days).

The shared `WinstonLogger` (`ts-libs/linea-shared-utils/src/logging/WinstonLogger.ts`) serializes metadata via `JSON.stringify` with no size cap, so any single oversized metadata value produces an unbounded log line.

## Goal

Guarantee no log line emitted by `WinstonLogger` can exceed a size that Loki will accept, regardless of which metadata field caused the bloat. This is a defense-in-depth backstop; it does not change the upstream code that produces the large value.

## Non-goals

- Not removing or trimming the `BeaconNodeApiClient` debug dumps (explicitly deferred by the user).
- Not dropping the `signature` field from the `PendingDeposit` type.
- Not changing prod `LOG_LEVEL` (a separate operational concern).
- Not avoiding the intermediate 18 MB string allocation inside `JSON.stringify`; that is a once-per-10-min memory blip, not the harm being fixed.

## Design

### Constants

Module-level named constants in `WinstonLogger.ts` (no raw string literals per workspace rule):

- `MAX_LOG_LINE_LENGTH = 250 * 1024` (256,000 characters). Sits under Loki's 262,144-byte entry limit with ~6 KB headroom for the timestamp/level/logger prefix and the truncation marker.
- A truncation marker suffix that reports the omitted character count, e.g. ` ... [truncated, +N chars]`.

### Unconditional default for all consumers

The cap is a hardcoded module constant applied unconditionally inside the shared `printf` formatter. It is **not** exposed as a `WinstonLoggerOptions` field and cannot be disabled per-consumer. Rationale: the Grafana Alloy sidecar and Loki backend are common infrastructure shared across every service that uses `WinstonLogger` (postman, native-yield automation service, shared-utils servers, etc.), so the line-size contract must hold for all consumers by default. Making it configurable would let one service opt out and reintroduce the same drop storm this fix prevents.

### Where the cap is applied

In the existing `printf` formatter, after the final `str` is fully assembled (`time=... level=... logger=... msg=...` + metadata + optional `error=...`). If `str.length > MAX_LOG_LINE_LENGTH`, the string is truncated so its total length equals `MAX_LOG_LINE_LENGTH` and ends with the marker. The marker reports `N = originalLength - (MAX_LOG_LINE_LENGTH - markerLength)` omitted characters.

This is a single chokepoint that catches any oversized value regardless of source: the 18 MB `returnVal` array, a future error payload, or any other metadata field.

### Why total-line cap (not per-value in `formatMetadataValue`)

A per-value cap cannot guarantee the total line stays under the limit when many medium-sized values sum together. The total-line cap is the robust backstop. The 18 MB intermediate string is still allocated by `JSON.stringify` before truncation, but that is acceptable per the non-goals.

### Unit of length

The cap is on string `.length` (UTF-16 code unit count). For the ASCII-heavy content this service emits (hex, JSON, logfmt), `.length` approximates byte length. The 6 KB headroom under Loki's byte limit absorbs minor multibyte overhead. This is a deliberate simplification; a byte-accurate `Buffer.byteLength` implementation is not warranted for this content profile.

## Tests

Added to `ts-libs/linea-shared-utils/src/logging/__tests__/WinstonLogger.test.ts`, following the existing in-memory `Writable` stream pattern:

1. A line under the limit is emitted unchanged (no marker present).
2. A line over the limit (metadata value larger than 250 KB) is truncated to exactly `MAX_LOG_LINE_LENGTH` characters and ends with the truncation marker.
3. The marker reports the correct number of omitted characters.

## Files changed

- `ts-libs/linea-shared-utils/src/logging/WinstonLogger.ts` - constants + truncation in `printf`.
- `ts-libs/linea-shared-utils/src/logging/__tests__/WinstonLogger.test.ts` - three new tests.

## Verification

- `pnpm -F @lfdt-lineth/shared-utils run test` passes (including the new tests).
- `pnpm -F @lfdt-lineth/shared-utils run lint` passes.

# Changelog

### Upcoming Breaking Changes

### Breaking Changes

### Additions and Improvements

- Formatting rules are aligned with Linea monorepo conventions by upgrading Spotless and ktlint and matching the shared editor configuration.
- Runtime and build dependencies are aligned with the same versions as the Linea `linea-monorepo` catalog (including Vert.x 5, Teku 25.12, Besu 26.5.0-36cda4f, Jackson, Netty, Tuweni `io.consensys.tuweni`, and related test libraries). SLF4J stays on the 2.0.x BOM so Vert.x 5 and Log4j’s SLF4J 2.x bridge resolve cleanly at runtime. Set `LINEA_LIBS_VERSION` when you need `build.linea.internal` artifacts to match a specific linea-monorepo build.

### Bug Fixes

- Maru validators are more resilient to duplicate QBFT gossip messages; already-seen messages no longer disrupt consensus processing.
- Restore execution-layer peering in the in-JVM Besu test harness (`BesuClusterTest` and downstream cluster-based integration tests). Recent Besu acceptance-DSL versions force DiscV5 on while supplying only enode bootnodes, leaving DiscV5 with no bootstrap records and `net_peerCount` stuck at `0`. The harness now opts the test runner back to DiscV4, which the existing enode-bootnode wiring already covers. Requires a Besu version that exposes `BesuNodeConfigurationBuilder.discoveryV5Enabled(boolean)`.
- Fixed an intermittent failure during the QBFT-to-PoS handover where the first post-merge `engine_forkchoiceUpdated` could be rejected by the execution client with `INVALID_PAYLOAD_ATTRIBUTES` (or `INVALID_WITHDRAWALS_PARAMS` on the V1 endpoint). The proposed `payloadAttributes.timestamp`, derived from `Math.round(wallClockMillis / 1000)`, could tie with the execution head's timestamp depending on sub-second wall-clock alignment. The next block's timestamp is now clamped to be strictly greater than the execution layer head's timestamp.
- Picked up a Besu fix (consumed via the `26.5.0-36cda4f` build) that makes `BftExecutors.awaitStop` tolerant of the IDLE state. Previously, shutting down a Besu node whose genesis declared QBFT/IBFT but ran PoS from block 0 (`terminalTotalDifficulty=0`) would NPE inside `BftMiningCoordinator.awaitStop` because the BFT timer/processor executors were never started. The NPE surfaced as `Error shutting down node …` and could leave the in-JVM Besu plugin context in an unrecoverable state for subsequent restarts.

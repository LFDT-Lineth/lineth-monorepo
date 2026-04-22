# Target checkpoints, L1 finalization, and API resume

Configure `[conflation.proof-aggregation]` in the coordinator TOML (for example `config/coordinator/coordinator-config-v2.toml` or
your Docker-mounted coordinator config). Optionally set `[api]` for JSON-RPC (default path `/`; `json-rpc-port` is often
`9546` in Docker, and that port is usually not published to the host).

## Block number targets: `target-end-blocks`

Example: `target-end-blocks = [4, 9]`, assuming other limits (deadline, traces, data size) do not cut batches earlier.

| Checkpoint (listed value) | Inclusive end of the conflated batch, blob, and aggregation |
|----------------------------|-------------------------------------------------------------|
| **4** | **4**                                                       |
| **9** | **9**                                                       |

```toml
[conflation.proof-aggregation]
target-end-blocks = [4, 9]
```

## Timestamp hard forks: `timestamp-based-hard-forks`

The list must be strictly ascending, with no duplicate timestamps. For example, `timestamp-based-hard-forks = [1700000100, 1700001000]` is two thresholds expressed as Unix seconds, or the same
instants as ISO-8601 strings in TOML.

| L2 block (example) | Block timestamp (example) | Inclusive end of the conflated batch, blob, and aggregation                                                       |
|----------------------|---------------------------|-------------------------------------------------------------------------------------------------------------------|
| 6 | `1700000000` | — |
| 7 | `1700000100` | The first threshold is crossed at block **7**, so the batch, blob, and aggregation are cut off with end block number **6**. |
| … | … | … |
| 15 | `1700001000` | The second threshold is crossed at block **15**, so the batch, blob, and aggregation are cut off with end block number **14**. |

```toml
[conflation.proof-aggregation]
timestamp-based-hard-forks = [1700000100, 1700001000]
# Or e.g. ["2023-11-14T22:15:00Z", "2023-11-14T22:30:00Z"] (same two instants) — must stay strictly ascending, no duplicates
```

## Block number targets and timestamp hard forks

Both can be set. The batch, blob, and aggregation are cut off at whichever target is hit first: a **block** end from `target-end-blocks` or a **time** threshold from `timestamp-based-hard-forks`. For
example, with `target-end-blocks = [4, 9]` and `timestamp-based-hard-forks = [1700000100, 1700001000]`:

- **Block 4** (the first **block** target) has timestamp `1700000000`, so the batch, blob, and aggregation in progress are cut off with end block number **4**.
- **Block 7** has timestamp `1700000100` (the first **time** threshold), so the cut-off is at end block number **6**.
- **Block 9** (the second **block** target) has timestamp `1700000500` (between the two time thresholds), so the cut-off is at end block number **9**.
- **Block 15** has timestamp `1700001000` (the second time threshold), so the cut-off is at end block number **14**.

## Wait on L1 finalization and on API: `wait-target-block-l1-finalization`, `wait-api-resume-after-target-block`

| Setting | If `true` |
|--------|------------|
| `wait-target-block-l1-finalization` | Do not import past the checkpoint until the L1-finalized L2 block height is high enough. |
| `wait-api-resume-after-target-block` | Before resuming, require the JSON-RPC call below. |
| Both `true` | Both conditions must pass before resuming. |
| Both `false` | No import pause from these options (all other behavior unchanged). |

```toml
[conflation.proof-aggregation]
wait-target-block-l1-finalization = true
wait-api-resume-after-target-block = true
```

## Resume: `conflation_signalTargetCheckpointResume`

```bash
docker compose -f docker/compose-tracing-v2.yml exec coordinator \
  curl -sS -X POST "http://127.0.0.1:9546/" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"conflation_signalTargetCheckpointResume","params":[]}'
```

`result: true` means the signal was applied (the API gate is on and a pause was waiting for it).

## See also

- [Get Started](get-started.md)
- [Local development guide](local-development-guide.md)

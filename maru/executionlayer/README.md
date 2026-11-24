# Execution layer

Maru's Engine API client, used to drive the execution-layer nodes (validator and followers).

## Engine API schemas in tests

The tests validate the requests the client sends, and the responses they feed to it, against the Engine API JSON schemas
of [ethereum/execution-apis](https://github.com/ethereum/execution-apis) (CC0-1.0).

The `downloadEngineApiSchemas` task in `build.gradle` downloads three files from the commit pinned in
`executionApisCommit` into `build/execution-apis/<commit>/`, only when they are not there yet. The commit is the one of
the release tag `v1.0.0-beta.7`, pinned by SHA because, unlike a tag, it cannot be moved:

- `src/schemas/base-types.yaml`
- `src/engine/openrpc/schemas/payload.yaml`
- `src/engine/openrpc/schemas/forkchoice.yaml`

The method signatures (`src/engine/openrpc/methods`) are not downloaded; `EngineApiSpec` maps each method to these
schemas instead.

## Supporting a new fork

1. Add the fork to `ElFork`, and any new fields Maru needs to `ExecutionPayload` / `PayloadAttributes` (`maru/core`).
2. Bump `executionApisCommit` in `build.gradle` to the commit of the first upstream release tag that defines the fork
   (an unreleased commit only if no tag does yet), and update the tag named next to it and above.
3. `EngineApiForkMethods`: map the fork to its `getPayload`, `newPayload` and `forkchoiceUpdated` versions, and to the
   `ExecutionPayload` / `PayloadAttributes` schema version.
4. Client code in `src/main/kotlin/maru/executionlayer/client`:
   - `EngineApiJson`: add the new fields, nullable so earlier forks omit them.
   - `EngineApiCodec`: gate the new fields and params on the method or schema version, in both directions.
5. `EngineApiSpec` (tests): map the new method versions to their params and result schemas.
6. `EngineApiTestData` (tests): add the new fields to the client inputs and the expected JSON.
7. Run `./gradlew :maru:executionlayer:test`. Every test parameterized over `ElFork` then covers the new fork,
   including spec validation of its requests and responses.

# Engine API schemas

Unmodified copies of the Engine API JSON schemas from
[ethereum/execution-apis](https://github.com/ethereum/execution-apis) (CC0-1.0), commit
`981aa66d79364be8322a9c358aaf0a571cca0873`. They are used by the tests to validate the requests the Engine API client
sends, and the responses the tests feed to it.

Files keep their upstream paths:

- `src/schemas/base-types.yaml`
- `src/engine/openrpc/schemas/payload.yaml`
- `src/engine/openrpc/schemas/forkchoice.yaml`

The method signatures (`src/engine/openrpc/methods`) are not copied because of their size; the tests map each method
to these schemas instead.

## Supporting a new fork

1. Add the fork to `ElFork`, and any new fields Maru needs to `ExecutionPayload` / `PayloadAttributes` (`maru/core`).
2. Refresh the schemas: copy the three files above from the upstream commit that defines the fork, keeping their paths,
   and update the commit hash in this README.
3. `EngineApiForkMethods`: map the fork to its `getPayload`, `newPayload` and `forkchoiceUpdated` versions, and to the
   `ExecutionPayload` / `PayloadAttributes` schema version.
4. Client code in `maru/executionlayer/src/main/kotlin/maru/executionlayer/client`:
   - `EngineApiJson`: add the new fields, nullable so earlier forks omit them.
   - `EngineApiCodec`: gate the new fields and params on the method or schema version, in both directions.
5. `EngineApiSpec` (tests): map the new method versions to their params and result schemas.
6. `EngineApiTestData` (tests): add the new fields to the client inputs and the expected JSON.
7. Run `./gradlew :maru:executionlayer:test`. Every test parameterized over `ElFork` then covers the new fork,
   including spec validation of its requests and responses.

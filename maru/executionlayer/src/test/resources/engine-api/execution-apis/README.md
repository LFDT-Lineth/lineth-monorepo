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

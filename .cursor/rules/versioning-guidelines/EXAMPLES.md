---
description: API and Asset Versioning - Examples
globs: "**/*.ts, **/*.js, **/*.kt, **/*.java, **/*.go, **/*.sol, **/*.py, **/*.abi, **/*.json"
alwaysApply: true
---

# API and Asset Versioning - WRONG / CORRECT Examples

## Method Versioning

```kotlin
// WRONG - breaking change to existing method signature
fun doSomething(items: List<Item>): Result {
    // changed from List to Map, breaking all callers
}

// CORRECT - new versioned method, old one deprecated
@Deprecated("please use doSomethingV2")
fun doSomethingV1(items: List<Item>): Result {
    // original implementation preserved
}

fun doSomethingV2(items: Map<String, Item>): Result {
    // new implementation with breaking signature
}
```

## Smart Contract / Solidity Versioning

```solidity
// WRONG - modifying existing interface that external consumers depend on
interface ILineaRollup {
    // changed parameter type, breaks all integrators
    function submitData(bytes32[] calldata data) external;
}

// CORRECT - new versioned interface
interface ILineaRollupV1 {
    function submitData(bytes calldata data) external;
}

interface ILineaRollupV2 {
    // new signature with breaking changes
    function submitData(bytes32[] calldata data) external;
}
```

## ABI / Asset Versioning

```
# WRONG - overwriting existing ABI consumed by deployed coordinator
contracts/
  ValidiumV1.abi    <-- modified in-place, breaks coordinator on next update

# CORRECT - new versioned asset alongside existing one
contracts/
  ValidiumV1.abi    <-- unchanged, coordinator continues to work
  ValidiumV2.abi    <-- new version with breaking changes
```

## TypeScript / JavaScript API Versioning

```typescript
// WRONG - renaming exported function that other packages import
export function getProof(blockNumber: number): Proof { ... }
// renamed to getProofWithValidation, breaking all importers

// CORRECT - deprecate and add new version
/** @deprecated Use getProofV2 instead */
export function getProofV1(blockNumber: number): Proof { ... }

export function getProofV2(blockNumber: number, options: ProofOptions): Proof { ... }
```

## REST / JSON-RPC API Versioning

```typescript
// WRONG - changing response shape of existing endpoint
app.get('/api/v1/status', (req, res) => {
    // changed from { status: string } to { state: string, details: object }
    // breaks all clients parsing the old shape
});

// CORRECT - new versioned endpoint
app.get('/api/v1/status', (req, res) => {
    // preserved for existing clients
    res.json({ status: 'ok' });
});

app.get('/api/v2/status', (req, res) => {
    // new shape for new clients
    res.json({ state: 'ok', details: { ... } });
});
```

## Configuration / JSON Asset Versioning

```
# WRONG - overwriting deployed config schema
config/
  coordinator-config.json    <-- changed schema, breaks deployed instances

# CORRECT - versioned config
config/
  coordinator-config-v1.json  <-- unchanged
  coordinator-config-v2.json  <-- new schema
```

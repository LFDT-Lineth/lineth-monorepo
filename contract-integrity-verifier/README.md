# Contract Integrity Verifier

A TypeScript tool to verify deployed smart contract integrity against local artifacts. Validates bytecode, ABI, and on-chain state.

Supports **multiple web3 libraries** (ethers, viem) via an adapter pattern.

## Package Structure

| Package | Description | Dependencies |
|---------|-------------|--------------|
| `@consensys/linea-contract-integrity-verifier` | Core library with adapter interface | None (pure TypeScript) |
| `@consensys/linea-contract-integrity-verifier-ethers` | Ethers v6 adapter + CLI | peer: `ethers >=6.0.0` |
| `@consensys/linea-contract-integrity-verifier-viem` | Viem adapter + CLI | peer: `viem >=2.22.0` |

## Installation

```bash
# Core + ethers adapter
pnpm add @consensys/linea-contract-integrity-verifier @consensys/linea-contract-integrity-verifier-ethers ethers

# Core + viem adapter
pnpm add @consensys/linea-contract-integrity-verifier @consensys/linea-contract-integrity-verifier-viem viem
```

## Usage

### With Ethers

```typescript
import { Verifier, loadConfig } from "@consensys/linea-contract-integrity-verifier";
import { EthersAdapter } from "@consensys/linea-contract-integrity-verifier-ethers";

const adapter = new EthersAdapter({ rpcUrl: "https://rpc.linea.build" });
const verifier = new Verifier(adapter);
const config = loadConfig("./config.json");

for (const contract of config.contracts) {
  const chain = config.chains[contract.chain];
  const result = await verifier.verifyContract(contract, chain, { verbose: true });
  console.log(result);
}
```

### With Viem

```typescript
import { Verifier, loadConfig } from "@consensys/linea-contract-integrity-verifier";
import { ViemAdapter } from "@consensys/linea-contract-integrity-verifier-viem";

const adapter = new ViemAdapter({ rpcUrl: "https://rpc.linea.build" });
const verifier = new Verifier(adapter);
const config = loadConfig("./config.json");

for (const contract of config.contracts) {
  const chain = config.chains[contract.chain];
  const result = await verifier.verifyContract(contract, chain, { verbose: true });
  console.log(result);
}
```

## CLI

Each adapter package includes a CLI tool:

```bash
# Using ethers adapter
npx verify-contract-ethers -c ./config.json -v

# Using viem adapter
npx verify-contract-viem -c ./config.json -v

# Or after building locally
cd contract-integrity-verifier/verifier-ethers
pnpm build
node dist/cli.mjs -c ../verifier-core/examples/configs/example.json -v
```

CLI Options:
- `-c, --config <PATH>` - Path to configuration file (required)
- `-v, --verbose` - Show detailed output
- `--contract <NAME>` - Filter to specific contract
- `--chain <NAME>` - Filter to specific chain
- `--skip-bytecode` - Skip bytecode verification
- `--skip-abi` - Skip ABI verification
- `--skip-state` - Skip state verification

## Configuration

Supports JSON and Markdown configuration formats.

### JSON Configuration

```json
{
  "chains": {
    "ethereum-sepolia": {
      "chainId": 11155111,
      "rpcUrl": "${ETHEREUM_SEPOLIA_RPC_URL}",
      "explorerUrl": "https://sepolia.etherscan.io"
    }
  },
  "contracts": [
    {
      "name": "MyContract-Proxy",
      "chain": "ethereum-sepolia",
      "address": "0x1234567890123456789012345678901234567890",
      "artifactFile": "../path/to/MyContract.json",
      "isProxy": true,
      "constructorArgs": ["0xMessageServiceAddress"],
      "stateVerification": {
        "viewCalls": [...],
        "slots": [...],
        "storagePaths": [...]
      }
    }
  ]
}
```

### Markdown Configuration

See `verifier-core/examples/configs/` for Markdown configuration examples.

## State Verification

State verification validates on-chain contract state after deployment or upgrade. The verifier supports **four verification methods**, each suited to different use cases:

### 1. View Calls (`viewCalls`)

Call public/external view functions and verify return values. **Best for:** values exposed via getter functions (roles, version strings, addresses).

```json
{
  "stateVerification": {
    "viewCalls": [
      {
        "function": "CONTRACT_VERSION",
        "expected": "7.0"
      },
      {
        "function": "hasRole",
        "params": [
          "0x76ef52a5344b10ed112c1d48c7c06f51e919518ea6fb005f9b25b359b955e3be",
          "0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1"
        ],
        "expected": true
      },
      {
        "function": "balanceOf",
        "params": ["0x..."],
        "expected": "1000000000000000000",
        "comparison": "gte"
      }
    ]
  }
}
```

**Fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `function` | string | ✓ | Function name (must exist in artifact ABI) |
| `params` | array | | Function arguments in order |
| `expected` | any | ✓ | Expected return value |
| `comparison` | string | | `eq` (default), `gt`, `gte`, `lt`, `lte`, `contains` |

### 2. Explicit Slots (`slots`)

Read raw storage slots directly. **Best for:** internal state not exposed via getters, packed storage variables, OpenZeppelin `_initialized` flag.

```json
{
  "stateVerification": {
    "slots": [
      {
        "slot": "0x0",
        "type": "uint8",
        "name": "_initialized",
        "expected": "7"
      },
      {
        "slot": "0x65",
        "type": "address",
        "name": "_admin",
        "expected": "0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1"
      },
      {
        "slot": "0x0",
        "type": "bool",
        "name": "_paused",
        "offset": 20,
        "expected": false
      }
    ]
  }
}
```

**Fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `slot` | string | ✓ | Storage slot as hex (e.g., `"0x0"`, `"0x65"`) |
| `type` | string | ✓ | Solidity type: `address`, `bool`, `uint8`-`uint256`, `int8`-`int256`, `bytes32` |
| `name` | string | ✓ | Variable name (for display) |
| `expected` | any | ✓ | Expected value |
| `offset` | number | | Byte offset for packed storage (0-31, default 0) |

**Common Slot Locations:**
| Pattern | Slot | Description |
|---------|------|-------------|
| OZ Initializable (v4) | `0x0` | `_initialized` (uint8) and `_initializing` (bool) packed |
| OZ Ownable (v4) | `0x33` | `_owner` address |
| OZ AccessControl | varies | Role mappings |
| EIP-1967 Implementation | `0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc` | Proxy implementation |
| EIP-1967 Admin | `0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103` | Proxy admin |

### 3. Storage Paths (`storagePaths`)

Access nested storage using schema-defined paths. **Best for:** ERC-7201 namespaced storage, complex struct fields, mapping values.

Requires a `schemaFile` that defines the storage layout.

```json
{
  "stateVerification": {
    "schemaFile": "../schemas/linea-rollup.json",
    "storagePaths": [
      {
        "path": "LineaRollupYieldExtensionStorage:_yieldManager",
        "expected": "0xafeB487DD3E3Cb0342e8CF0215987FfDc9b72c9b"
      },
      {
        "path": "YieldManagerStorage:targetWithdrawalReservePercentageBps",
        "expected": "8000"
      },
      {
        "path": "TokenStorage:_balances[0x1234...]",
        "expected": "1000000000000000000",
        "comparison": "gte"
      }
    ]
  }
}
```

**Path Syntax:**
- Simple field: `StructName:fieldName`
- Mapping access: `StructName:mappingField[key]`
- Nested mapping: `StructName:nestedMap[key1][key2]`

**Fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | ✓ | Storage path (see syntax above) |
| `expected` | any | ✓ | Expected value |
| `comparison` | string | | `eq` (default), `gt`, `gte`, `lt`, `lte` |

### 4. Namespaces (`namespaces`)

Verify multiple variables within an ERC-7201 namespace. **Best for:** batch verification of related variables in namespaced storage.

```json
{
  "stateVerification": {
    "namespaces": [
      {
        "id": "linea.storage.YieldManager",
        "variables": [
          { "offset": 0, "type": "address", "name": "owner", "expected": "0x..." },
          { "offset": 1, "type": "uint256", "name": "totalDeposits", "expected": "0" },
          { "offset": 2, "type": "bool", "name": "paused", "expected": false }
        ]
      }
    ]
  }
}
```

**Fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | ✓ | ERC-7201 namespace ID (e.g., `linea.storage.YieldManager`) |
| `variables` | array | ✓ | Variables to verify |
| `variables[].offset` | number | ✓ | Slot offset from namespace base (0, 1, 2, ...) |
| `variables[].type` | string | ✓ | Solidity type |
| `variables[].name` | string | ✓ | Variable name (for display) |
| `variables[].expected` | any | ✓ | Expected value |

### Choosing the Right Method

| Use Case | Method | Example |
|----------|--------|---------|
| Contract version string | `viewCalls` | `CONTRACT_VERSION()` |
| Access control roles | `viewCalls` | `hasRole(ADMIN_ROLE, addr)` |
| OZ initialized flag | `slots` | Slot `0x0`, type `uint8` |
| ERC-7201 struct field | `storagePaths` | `MyStorage:myField` |
| Mapping value | `storagePaths` | `MyStorage:balances[addr]` |
| Multiple related vars | `namespaces` | Batch verify namespace |

### Markdown Table Format

For Markdown configs, use a table with these columns:

```markdown
| Type | Description | Check | Params | Expected |
|------|-------------|-------|--------|----------|
| viewCall | Contract version | `CONTRACT_VERSION` | | `7.0` |
| viewCall | Admin role check | `hasRole` | `0x00...`,`0xe6Ec...` | true |
| slot | Initialized version | `0x0` | uint8 | `7` |
| storagePath | Yield manager | `LineaRollupStorage:_yieldManager` | | `0xafeB...` |
```

**Column Meanings by Type:**
| Type | Check Column | Params Column | Expected Column |
|------|--------------|---------------|-----------------|
| `viewCall` | Function name | Comma-separated args | Return value |
| `slot` | Slot hex (`0x0`) | Type (`uint8`) | Slot value |
| `storagePath` | Path (`Struct:field`) | (unused) | Value |

## Web3Adapter Interface

The adapter pattern allows the core library to work with any web3 library:

```typescript
interface Web3Adapter {
  // Crypto (synchronous)
  keccak256(value: string | Uint8Array): string;
  checksumAddress(address: string): string;
  readonly zeroAddress: string;

  // ABI (synchronous)
  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string;
  encodeFunctionData(abi: readonly AbiElement[], functionName: string, args?: readonly unknown[]): string;
  decodeFunctionResult(abi: readonly AbiElement[], functionName: string, data: string): readonly unknown[];

  // RPC (asynchronous)
  getCode(address: string): Promise<string>;
  getStorageAt(address: string, slot: string): Promise<string>;
  call(to: string, data: string): Promise<string>;
}
```

## Tools

The `verifier-core` package includes command-line tools for working with artifacts and storage schemas.

### Artifact Converter

Convert between Hardhat and Foundry artifact formats. Auto-detects input format.

```bash
cd verifier-core

# Auto-detect and convert to opposite format
npx ts-node tools/convert-artifact.ts <input.json> <output.json>

# Force conversion to specific format
npx ts-node tools/convert-artifact.ts <input.json> <output.json> --to-hardhat
npx ts-node tools/convert-artifact.ts <input.json> <output.json> --to-foundry
```

**Examples:**

```bash
# Convert Foundry artifact to Hardhat format
npx ts-node tools/convert-artifact.ts \
  ../contracts/out/MyContract.sol/MyContract.json \
  ./artifacts/MyContract.hardhat.json \
  --to-hardhat

# Convert Hardhat artifact to Foundry format
npx ts-node tools/convert-artifact.ts \
  ../contracts/artifacts/MyContract.json \
  ./artifacts/MyContract.foundry.json \
  --to-foundry
```

**Options:**
- `--to-hardhat` - Force conversion to Hardhat format
- `--to-foundry` - Force conversion to Foundry format
- (no flag) - Auto-detect input and convert to opposite format

### View Calls Generator

Generate a template of view call configurations from a contract ABI. Extracts all `view`/`pure` functions.

```bash
cd verifier-core

# Generate all view functions
npx ts-node tools/generate-viewcalls.ts \
  ../contracts/out/MyContract.sol/MyContract.json \
  viewcalls-template.json

# Only functions without parameters (simpler to verify)
npx ts-node tools/generate-viewcalls.ts \
  ../contracts/out/MyContract.sol/MyContract.json \
  viewcalls-template.json \
  --no-params
```

**Output Example:**
```json
{
  "viewCalls": [
    { "$comment": "Get constant CONTRACT_VERSION", "function": "CONTRACT_VERSION", "expected": "TODO_string" },
    { "$comment": "Check if account has role", "function": "hasRole", "params": ["TODO_role_bytes32", "TODO_account_address"], "expected": "TODO_bool" }
  ]
}
```

**Options:**
- `--no-params` - Only include functions without parameters
- `--no-comments` - Exclude `$comment` fields

### Initializer Analyzer

Analyze constructor and initializer functions to suggest state verifications. Helps identify what state should be verified after deployment/upgrade.

```bash
cd verifier-core

# Analyze and print suggestions to console
npx ts-node tools/analyze-initializers.ts \
  ../contracts/out/LineaRollup.sol/LineaRollup.json

# Save analysis to file
npx ts-node tools/analyze-initializers.ts \
  ../contracts/out/LineaRollup.sol/LineaRollup.json \
  analysis.json
```

**What it detects:**
- Constructor parameters
- `initialize()` functions
- `reinitializeV*()` functions
- Address inputs that suggest role grants
- Config values that should be verified

**Limitations:**
- Only analyzes function signatures (ABI), not implementation
- Cannot determine exact role hashes or storage slots
- User must fill in expected values from deployment scripts

### Storage Schema Generator

Generate storage schema JSON from Solidity storage layout files. Parses struct definitions and ERC-7201 namespace annotations.

```bash
cd verifier-core

npx ts-node tools/generate-schema.ts <input.sol> <output.json> [--verbose]
```

**Examples:**

```bash
# Generate schema from YieldManager storage layout
npx ts-node tools/generate-schema.ts \
  ../../contracts/src/yield/YieldManagerStorageLayout.sol \
  examples/schemas/yield-manager.json

# With verbose output to see field details
npx ts-node tools/generate-schema.ts \
  ../../contracts/src/LineaRollupStorage.sol \
  examples/schemas/linea-rollup.json \
  --verbose
```

**Options:**
- `--verbose, -v` - Show detailed field-level output

**Note:** This tool requires either `ethers` or `viem` to be installed for ERC-7201 slot calculations.

**Solidity Annotations:**

The generator recognizes ERC-7201 namespace annotations in NatSpec comments:

```solidity
/// @custom:storage-location erc7201:linea.storage.YieldManager
struct YieldManagerStorage {
    address yieldProvider;
    uint256 totalYield;
    mapping(address => uint256) userBalances;
}
```

This produces a schema with computed `baseSlot` for the namespace:

```json
{
  "structs": {
    "YieldManagerStorage": {
      "namespace": "linea.storage.YieldManager",
      "baseSlot": "0x594904a11ae10ad7613c91ac3c92c7c3bba397934d377ce6d3e0aaffbc17df00",
      "fields": {
        "yieldProvider": { "slot": 0, "type": "address" },
        "totalYield": { "slot": 1, "type": "uint256" },
        "userBalances": { "slot": 2, "type": "mapping(address => uint256)" }
      }
    }
  }
}
```

## Project Structure

```
contract-integrity-verifier/
├── verifier-core/                    # @consensys/linea-contract-integrity-verifier
│   ├── src/
│   │   ├── adapter.ts               # Web3Adapter interface
│   │   ├── config.ts                # Config loading (JSON + Markdown)
│   │   ├── index.ts                 # Public exports
│   │   ├── types.ts                 # All TypeScript types
│   │   ├── verifier.ts              # Main Verifier class
│   │   └── utils/
│   │       ├── abi.ts               # ABI utilities
│   │       ├── bytecode.ts          # Bytecode comparison
│   │       ├── markdown-config.ts   # Markdown config parser
│   │       └── storage.ts           # ERC-7201 storage utilities
│   ├── tests/
│   │   └── run-tests.ts             # Test suite
│   ├── tools/
│   │   ├── analyze-initializers.ts  # Initializer analysis for verification suggestions
│   │   ├── convert-artifact.ts      # Artifact format converter
│   │   ├── generate-schema.ts       # Storage schema generator
│   │   └── generate-viewcalls.ts    # View call template generator
│   └── examples/                    # Example configs and schemas
├── verifier-ethers/                  # @consensys/linea-contract-integrity-verifier-ethers
│   └── src/
│       ├── index.ts                 # EthersAdapter
│       └── cli.ts                   # CLI using ethers
└── verifier-viem/                    # @consensys/linea-contract-integrity-verifier-viem
    └── src/
        ├── index.ts                 # ViemAdapter
        └── cli.ts                   # CLI using viem
```

## Development

```bash
# Build all packages (order matters - core first)
cd contract-integrity-verifier/verifier-core && pnpm build
cd ../verifier-ethers && pnpm build
cd ../verifier-viem && pnpm build

# Or build all at once
pnpm --filter "@consensys/linea-contract-integrity-verifier" build
pnpm --filter "@consensys/linea-contract-integrity-verifier-ethers" build
pnpm --filter "@consensys/linea-contract-integrity-verifier-viem" build

# Typecheck
cd verifier-core && npx tsc --noEmit

# Lint
cd verifier-core && pnpm lint:fix
```

## Testing

### Unit Tests (Mock Adapter)

Unit tests use mock adapters and don't require network access:

```bash
# Core package - unit tests
cd verifier-core && pnpm test

# Adapter packages - integration tests with mock RPC
cd verifier-ethers && pnpm test:integration
cd verifier-viem && pnpm test:integration
```

### Live Integration Tests

Live tests connect to real networks and verify against deployed contracts.
Requires `ETHEREUM_SEPOLIA_RPC_URL` environment variable:

```bash
# Set RPC URL
export ETHEREUM_SEPOLIA_RPC_URL="https://sepolia.infura.io/v3/YOUR_KEY"

# Run live tests with ethers adapter
cd verifier-ethers && pnpm test:live

# Run live tests with viem adapter
cd verifier-viem && pnpm test:live
```

Live tests will skip gracefully if the environment variable is not set.

### Test Artifacts

The packages use real Hardhat artifacts from `contracts/deployments/bytecode/` for live tests.
Mock artifacts in `tests/fixtures/artifacts/` are used for offline unit tests.

## Features

- **Bytecode Verification**: Compare deployed bytecode against local artifacts
- **Immutable Detection**: Automatically detect and validate immutable values
- **ABI Verification**: Validate function selectors match artifact ABI
- **State Verification**: Verify on-chain state (storage slots, view calls)
- **ERC-7201 Support**: Compute and verify namespaced storage slots
- **Artifact Support**: Works with both Hardhat and Foundry artifacts
- **Markdown Config**: Human-readable configuration files
- **Multiple Web3 Libraries**: Use ethers or viem via adapter pattern

## Security Considerations

- **Input Validation**: All user inputs (addresses, paths, config values) are validated
- **Path Traversal**: File paths are resolved relative to config file location
- **Environment Variables**: Sensitive values (RPC URLs) should be passed via environment variables
- **No Secrets in Code**: Never commit RPC URLs or private keys to configuration files

# Besu Plugins

> Kotlin/Java plugins extending Besu for Linea sequencer and state recovery functionality.

> **Diagrams:** [Besu Plugins Architecture](../diagrams/besu-plugins-architecture.mmd) | [Plugin Lifecycle](../diagrams/plugin-lifecycle.mmd) | [Sequencer Architecture](../diagrams/sequencer-architecture.mmd)

## Overview

Two main plugin groups:
- **linea-sequencer**: Transaction selection, validation, and custom RPC endpoints
- **state-recovery**: Rebuild L2 state from L1 submission data

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                           BESU PLUGINS                                 │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                    linea-sequencer                               │  │
│  │                                                                  │  │
│  │  ┌───────────────────┐ ┌───────────────────┐ ┌─────────────────┐ │  │
│  │  │ Transaction       │ │ Transaction Pool  │ │ Extra Data      │ │  │
│  │  │ Selector Plugin   │ │ Validator Plugin  │ │ Plugin          │ │  │
│  │  │                   │ │                   │ │                 │ │  │
│  │  │ - Trace limits    │ │ - Gas limits      │ │ - Block pricing │ │  │
│  │  │ - Block gas       │ │ - Call data size  │ │ - Extra data    │ │  │
│  │  │ - Profitability   │ │ - Profitability   │ │   management    │ │  │
│  │  │ - Bundle handling │ │ - Simulation      │ │                 │ │  │
│  │  └───────────────────┘ └───────────────────┘ └─────────────────┘ │  │
│  │                                                                  │  │
│  │  ┌───────────────────┐ ┌───────────────────┐ ┌─────────────────┐ │  │
│  │  │ Gas Estimation    │ │ Bundle Endpoints  │ │ Forced Tx       │ │  │
│  │  │ Endpoint Plugin   │ │ Plugin            │ │ Endpoints       │ │  │
│  │  │                   │ │                   │ │ Plugin          │ │  │
│  │  │ linea_estimateGas │ │ linea_sendBundle  │ │ linea_sendForced│ │  │
│  │  │                   │ │ linea_cancelBundle│ │ RawTransaction  │ │  │
│  │  └───────────────────┘ └───────────────────┘ └─────────────────┘ │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                    state-recovery                                │  │
│  │                                                                  │  │
│  │  ┌────────────────────────────────────────────────────────────┐  │  │
│  │  │               LineaStateRecoveryPlugin                     │  │  │
│  │  │                                                            │  │  │
│  │  │ ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │  │  │
│  │  │ │ L1 Event    │  │ BlobScan    │  │ Block Importer      │  │  │  │
│  │  │ │ Monitor     │  │ Client      │  │                     │  │  │  │
│  │  │ │             │  │             │  │ - Decompress blob   │  │  │  │
│  │  │ │ Watch       │  │ Fetch blob  │  │ - Import blocks     │  │  │  │
│  │  │ │ LineaRollup │  │ data        │  │ - Verify state      │  │  │  │
│  │  │ └─────────────┘  └─────────────┘  └─────────────────────┘  │  │  │
│  │  │                                                            │  │  │
│  │  └────────────────────────────────────────────────────────────┘  │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
besu-plugins/
├── linea-sequencer/
│   ├── sequencer/
│   │   └── src/main/java/
│   │       └── net/consensys/linea/sequencer/
│   │           ├── txselection/
│   │           │   └── LineaTransactionSelectorPlugin.java
│   │           ├── txpoolvalidation/
│   │           │   └── LineaTransactionPoolValidatorPlugin.java
│   │           ├── txvalidation/
│   │           │   └── LineaTransactionValidatorPlugin.java
│   │           ├── extradata/
│   │           │   └── LineaExtraDataPlugin.java
│   │           ├── rpc/
│   │           │   ├── LineaEstimateGasEndpointPlugin.java
│   │           │   ├── LineaBundleEndpointsPlugin.java
│   │           │   ├── LineaForcedTransactionEndpointsPlugin.java
│   │           │   └── LineaSetExtraDataEndpointPlugin.java
│   │           └── ForwardBundlesPlugin.java
│   └── acceptance-tests/
│       └── src/test/kotlin/          # Integration tests
│
└── state-recovery/                   # Written in Kotlin
    ├── appcore/
    │   ├── domain-models/            # Domain entities
    │   ├── clients-interfaces/       # Client abstractions
    │   └── logic/                    # Core recovery logic
    ├── besu-plugin/                  # Besu plugin entry point
    ├── clients/
    │   ├── blobscan-client/          # BlobScan HTTP client
    │   └── eth-api/                  # Ethereum API client
    └── test-cases/                   # Integration/E2E tests
```

> **Note**: The linea-sequencer plugins are written in **Java**, while state-recovery is written in **Kotlin**.

## Linea Sequencer Plugins

### 1. LineaTransactionSelectorPlugin

Controls which transactions are included in blocks.

**Selectors:**
- `TraceLineLimitTransactionSelector` - Enforces trace line limits
- `MaxBlockGasTransactionSelector` - Block gas limit
- `MaxBlockCallDataTransactionSelector` - Call data size limit
- `ProfitableTransactionSelector` - Minimum profitability
- `BundleConstraintTransactionSelector` - Bundle ordering

```kotlin
class LineaTransactionSelectorPlugin : AbstractLineaSharedPrivateOptionsPlugin() {
    
    override fun createTransactionSelector(
        configuration: TransactionSelectorConfiguration
    ): PluginTransactionSelector {
        return CompositeTransactionSelector(
            listOf(
                TraceLineLimitTransactionSelector(traceLimitsConfig),
                MaxBlockGasTransactionSelector(maxBlockGas),
                MaxBlockCallDataTransactionSelector(maxCallDataSize),
                ProfitableTransactionSelector(profitabilityConfig),
                BundleConstraintTransactionSelector(bundleConfig)
            )
        )
    }
}
```

### 2. LineaTransactionPoolValidatorPlugin

Validates transactions before pool admission.

**Validators:**
- Gas limit validation
- Call data size check
- Profitability check
- Transaction simulation
- Denied address check (sender, recipient, and EIP-7702 authorization list entries)

```kotlin
class LineaTransactionPoolValidatorPlugin : AbstractLineaSharedPrivateOptionsPlugin() {
    
    override fun createTransactionPoolValidator(
        configuration: TransactionPoolValidatorConfiguration
    ): PluginTransactionPoolValidator {
        return CompositeValidator(
            listOf(
                GasLimitValidator(maxTxGas),
                CalldataValidator(maxCallDataSize),
                ProfitabilityValidator(minProfit),
                SimulationValidator(simulationService),
                DeniedAddressValidator(deniedAddresses)
            )
        )
    }
}
```

### 3. LineaEstimateGasEndpointPlugin

Custom gas estimation with Linea-specific checks.

```kotlin
@RpcMethod("linea_estimateGas")
fun estimateGas(
    callParams: CallParameter,
    blockParam: BlockParameter
): JsonRpcResponse {
    // Estimate gas
    val gasEstimate = estimateGasService.estimate(callParams)
    
    // Validate trace line count
    val lineCount = traceService.getLineCount(callParams)
    if (lineCount > maxLineCount) {
        return JsonRpcError.TRACE_LIMIT_EXCEEDED
    }
    
    return JsonRpcSuccess(gasEstimate)
}
```

### 4. LineaBundleEndpointsPlugin

Transaction bundle management.

```kotlin
@RpcMethod("linea_sendBundle")
fun sendBundle(
    bundleParams: BundleParams
): JsonRpcResponse {
    // Validate bundle
    validateBundle(bundleParams)
    
    // Add to bundle pool
    bundlePoolService.addBundle(bundle)
    
    return JsonRpcSuccess(bundleId)
}

@RpcMethod("linea_cancelBundle")
fun cancelBundle(bundleId: String): JsonRpcResponse {
    bundlePoolService.cancelBundle(bundleId)
    return JsonRpcSuccess(true)
}
```

### 5. LineaExtraDataPlugin

Manages block extra data for pricing.

```kotlin
class LineaExtraDataPlugin : AbstractLineaSharedPrivateOptionsPlugin() {
    
    override fun createExtraDataProvider(): ExtraDataProvider {
        return LineaExtraDataProvider(pricingConfig)
    }
}
```

## State Recovery Plugin

### Purpose

Rebuild L2 state by replaying L1 submission data.

### Recovery Flow

```
┌────────────────────────────────────────────────────────────────────────┐
│                      STATE RECOVERY FLOW                               │
│                                                                        │
│  1. Monitor L1              2. Fetch Blobs           3. Import         │
│  ┌─────────────────┐        ┌─────────────────┐      ┌──────────────┐  │
│  │ Watch           │        │ BlobScan        │      │ Decompress   │  │
│  │ LineaRollup     │───────▶│ Client          │─────▶│ & Deserialize│  │
│  │ DataSubmittedV3 │        │                 │      │              │  │
│  │ events          │        │ GET /blobs/{id} │      │ Import via   │  │
│  └─────────────────┘        └─────────────────┘      │ BlockImporter│  │
│                                                      └──────┬───────┘  │
│                                                             │          │
│  4. Verify State            5. Continue/Stop                │          │
│  ┌─────────────────┐        ┌─────────────────┐             │          │
│  │ State Manager   │◀───────│ Check target    │◀────────────┘          │
│  │ (Shomei)        │        │ block reached?  │                        │
│  │                 │        │                 │                        │
│  │ Verify state    │        │ Yes → Stop P2P  │                        │
│  │ root matches    │        │ No → Continue   │                        │
│  └─────────────────┘        └─────────────────┘                        │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### Plugin Implementation

```kotlin
class LineaStateRecoveryPlugin : BesuPlugin {
    
    override fun register(context: BesuContext) {
        // Get required services
        blockchainService = context.getService(BlockchainService::class.java)
        syncService = context.getService(SynchronizationService::class.java)
        
        // Register CLI options
        registerOptions()
    }
    
    override fun start() {
        // Initialize recovery service
        val recoveryService = StateRecoveryService(
            l1Client = L1EthApiClient(l1RpcUrl),
            blobscanClient = BlobScanClient(blobscanUrl),
            blockImporter = BesuBlockImporter(blockchainService),
            stateManager = ShomeiStateManager(shomeiUrl)
        )
        
        // Start recovery from configured block
        recoveryService.startRecovery(
            startBlockNumber = config.startBlock,
            targetBlockNumber = config.targetBlock
        )
    }
}
```

### Architecture

```
state-recovery/
├── appcore/
│   ├── domain-models/
│   │   ├── SubmissionEvent.kt      # L1 submission event
│   │   ├── BlobData.kt             # Blob content
│   │   └── RecoveryState.kt        # Recovery progress
│   │
│   ├── clients-interfaces/
│   │   ├── IL1Client.kt            # L1 RPC interface
│   │   ├── IBlobscanClient.kt      # BlobScan interface
│   │   └── IStateManager.kt        # Shomei interface
│   │
│   └── logic/
│       ├── StateRecoveryService.kt # Main coordinator
│       ├── SubmissionsFetcher.kt   # Fetch L1 submissions
│       ├── BlobDecompressor.kt     # Decompress blobs
│       └── BlockImporter.kt        # Import blocks
│
├── clients/
│   ├── blobscan-client/
│   │   └── BlobScanApiClient.kt
│   └── eth-api/
│       └── L1EthApiClient.kt
│
└── besu-plugin/
    └── LineaStateRecoveryPlugin.kt  # Plugin entry point
```

## Plugin Lifecycle

```
┌────────────────────────────────────────────────────────────────────────┐
│                        PLUGIN LIFECYCLE                                │
│                                                                        │
│  1. Discovery            2. Register              3. Start             │
│  ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐   │
│  │ @AutoService    │     │ register()      │     │ start()         │   │
│  │ (BesuPlugin)    │────▶│                 │────▶│                 │   │
│  │                 │     │ - Get services  │     │ - Initialize    │   │
│  │ META-INF/       │     │ - Register CLI  │     │ - Start workers │   │
│  │ services/       │     │ - Register RPC  │     │ - Begin         │   │
│  │                 │     │                 │     │   processing    │   │
│  └─────────────────┘     └─────────────────┘     └─────────────────┘   │
│                                                                        │
│  4. Running              5. Stop                                       │
│  ┌─────────────────┐     ┌─────────────────┐                           │
│  │ Process blocks  │     │ stop()          │                           │
│  │ Handle requests │────▶│                 │                           │
│  │ Validate txs    │     │ - Cleanup       │                           │
│  │                 │     │ - Close conns   │                           │
│  └─────────────────┘     └─────────────────┘                           │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Shared Infrastructure

### AbstractLineaSharedPrivateOptionsPlugin

Base class providing common functionality:

```kotlin
abstract class AbstractLineaSharedPrivateOptionsPlugin : BesuPlugin {
    
    // Shared services
    protected lateinit var blockchainService: BlockchainService
    protected lateinit var worldStateService: WorldStateService
    protected lateinit var metricsSystem: MetricsSystem
    protected lateinit var bundlePoolService: BundlePoolService
    
    // Shared configuration
    protected lateinit var transactionSelectorConfig: TransactionSelectorConfig
    protected lateinit var poolValidatorConfig: PoolValidatorConfig
    protected lateinit var profitabilityConfig: ProfitabilityConfig
    
    override fun register(context: BesuContext) {
        // Initialize shared services
        blockchainService = context.getService(BlockchainService::class.java)
        worldStateService = context.getService(WorldStateService::class.java)
        // ...
    }
}
```

## Building

```bash
# Build all plugins
./gradlew :besu-plugins:linea-sequencer:build
./gradlew :besu-plugins:state-recovery:besu-plugin:build

# Run tests
./gradlew :besu-plugins:linea-sequencer:test
./gradlew :besu-plugins:state-recovery:test-cases:test

# Run acceptance tests
./gradlew :besu-plugins:linea-sequencer:acceptance-tests:test
```

## Configuration

### Sequencer Plugin Options

```bash
--plugin-linea-max-tx-gas-limit=30000000
--plugin-linea-max-call-data-size=30720
--plugin-linea-denied-addresses=0x...
--plugin-linea-profitability-min-margin=0.01
--plugin-linea-trace-limits-file=/config/traces-limits-v2.toml
```

### State Recovery Plugin Options

```bash
--plugin-linea-state-recovery-l1-rpc-url=http://l1-node:8545
--plugin-linea-state-recovery-blobscan-url=http://blobscan:4001
--plugin-linea-state-recovery-start-block=0
--plugin-linea-state-recovery-target-block=latest
```

## Dependencies

**linea-sequencer:**
- `:tracer:arithmetization` - Trace processing
- `build.linea:blob-compressor` - Compression

**state-recovery:**
- `:jvm-libs:linea:clients:linea-contract-clients`
- `:jvm-libs:linea:web3j-extensions`
- `:jvm-libs:linea:blob-decompressor`

# E2E Tests

> End-to-end test suite validating the complete Linea stack.

> **Diagram:** [E2E Test Coverage](../diagrams/e2e-test-coverage.mmd) (Mermaid source)

## Overview

The E2E tests validate:
- Cross-chain bridging (L1 ↔ L2)
- Message passing and claiming
- Rollup submission and finalization
- Coordinator restart recovery
- Node fleet consistency
- Liveness detection

## Test Coverage

```
┌────────────────────────────────────────────────────────────────────────┐
│                         E2E TEST COVERAGE                              │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                       Bridging Tests                             │  │
│  │                                                                  │  │
│  │  bridge-tokens.spec.ts                                           │  │
│  │  ├── L1 → L2: mint, approve, bridge, anchor, claim               │  │
│  │  └── L2 → L1: mint, approve, bridge, finalize, claim             │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                      Messaging Tests                             │  │
│  │                                                                  │  │
│  │  messaging.spec.ts                                               │  │
│  │  ├── L1 → L2: with/without fees, with/without calldata           │  │
│  │  ├── L2 → L1: with/without fees, with/without calldata           │  │
│  │  └── Postman sponsoring                                          │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                       Rollup Tests                               │  │
│  │                                                                  │  │
│  │  submission-finalization.spec.ts                                 │  │
│  │  ├── L2 anchoring via RollingHashUpdated                         │  │
│  │  ├── L1 data submission (DataSubmittedV3)                        │  │
│  │  ├── Finalization with proofs (DataFinalizedV3)                  │  │
│  │  └── Safe/finalized tag updates                                  │  │
│  │                                                                  │  │
│  │  restart.spec.ts                                                 │  │
│  │  ├── Coordinator restart recovery                                │  │
│  │  └── Anchoring resume after restart                              │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                   Infrastructure Tests                           │  │
│  │                                                                  │  │
│  │  linea-besu-fleet.spec.ts                                        │  │
│  │  ├── Leader/follower consistency                                 │  │
│  │  ├── linea_estimateGas matching                                  │  │
│  │  └── Block sync verification                                     │  │
│  │                                                                  │  │
│  │  liveness.spec.ts                                                │  │
│  │  └── Sequencer downtime detection                                │  │
│  │                                                                  │  │
│  │  l2.spec.ts                                                      │  │
│  │  ├── Transaction type support                                    │  │
│  │  └── Calldata size limits                                        │  │
│  │                                                                  │  │
│  │  opcodes.spec.ts                                                 │  │
│  │  └── EVM opcode execution                                        │  │
│  │                                                                  │  │
│  │  send-bundle.spec.ts                                             │  │
│  │  └── Transaction bundling                                        │  │
│  │                                                                  │  │
│  │  transaction-exclusion.spec.ts                                   │  │
│  │  └── Rejection tracking                                          │  │
│  │                                                                  │  │
│  │  shomei-get-proof.spec.ts                                        │  │
│  │  └── Merkle proof generation                                     │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Test Suite Logic

This section explains the main steps and validation logic for each test suite.

### messaging.spec.ts — Cross-Layer Message Passing

Tests L1↔L2 message sending and automatic claiming by the Postman service.

**Test: L1 → L2 with fee and calldata**
1. Generate fresh L1 and L2 accounts with funded balances
2. Encode calldata targeting `DummyContract.setPayload()` on L2
3. Call `LineaRollup.sendMessage()` on L1 with fee and calldata
4. Extract `MessageSent` event and capture `messageHash`
5. Poll L2 `L2MessageService` for `MessageClaimed` event matching the hash
6. Assert: Message was automatically claimed on L2

**Test: L1 → L2 without fee (Postman sponsoring)**
1. Same flow but with `fee = 0`
2. Validates that Postman sponsors gas for zero-fee messages

**Test: L2 → L1 with fee and calldata**
1. Generate accounts on both layers
2. Call `L2MessageService.sendMessage()` on L2
3. Wait for `L2MessagingBlockAnchored` event on L1 (confirms L2 block was finalized)
4. Poll L1 `LineaRollup` for `MessageClaimed` event
5. Assert: Message was claimed on L1 after finalization

---

### bridge-tokens.spec.ts — ERC20 Token Bridging

Tests the complete token bridge flow in both directions.

**Test: Bridge L1 → L2**
1. Generate L1 and L2 accounts
2. Mint test ERC20 tokens to L1 account
3. Approve `L1TokenBridge` to spend tokens
4. Call `L1TokenBridge.bridgeToken()` with recipient = L2 account
5. Extract `MessageSent` event from transaction
6. Wait for `RollingHashUpdated` on L2 (anchoring complete)
7. Verify `inboxL1L2MessageStatus` shows message is anchored
8. Wait for `MessageClaimed` event on L2
9. Wait for `NewTokenDeployed` event (bridged token creation)
10. Assert: L2 bridged token balance equals bridged amount

**Test: Bridge L2 → L1**
1. Generate accounts, mint tokens on L2
2. Approve `L2TokenBridge`, call `bridgeToken()`
3. Wait for `MessageClaimed` on L1 LineaRollup
4. Wait for `NewTokenDeployed` on L1 TokenBridge
5. Assert: L1 bridged token balance equals bridged amount

---

### submission-finalization.spec.ts — Rollup Data Flow

Tests the coordinator's blob submission and proof finalization pipeline.

**Test: L2 Anchoring**
1. Send multiple messages on L1 to generate anchoring work
2. Capture the last message number from `MessageSent` events
3. Poll L2 for `RollingHashUpdated` event where `messageNumber >= lastMessageNumber`
4. Query L1 `rollingHashes(messageNumber)` and L2 `lastAnchoredL1MessageNumber()`
5. Assert: Rolling hashes match and anchored number is updated

**Test: L1 Data Submission and Finalization**
1. Get current finalized L2 block number from `LineaRollup.currentL2BlockNumber()`
2. Wait for `DataSubmittedV3` event (blob submission)
3. Wait for `DataFinalizedV3` event with `startBlockNumber > currentL2BlockNumber`
4. Query new state root hash from `stateRootHashes(endBlockNumber)`
5. Assert: Finalized block number increased, state root matches event

**Test: Safe/Finalized Tag Update**
1. Query sequencer endpoint for `eth_getBlockByNumber("safe")` and `"finalized"`
2. Poll until both return block numbers >= last finalized on L1
3. Assert: Sequencer correctly reflects L1 finalization state

---

### restart.spec.ts — Coordinator Recovery

Tests that the coordinator resumes correctly after restart.

**Test: Resume Blob Submission After Restart**
1. Wait for initial `DataSubmittedV3` and `DataFinalizedV3` events
2. Synchronize with parallel test using mutex pattern
3. Execute `docker restart coordinator`
4. Wait for new `DataSubmittedV3` event after restart
5. Wait for new `DataFinalizedV3` with `endBlockNumber > previous`
6. Assert: Finalization continued past pre-restart state

**Test: Resume Anchoring After Restart**
1. Send 5 L1→L2 messages before restart
2. Wait for `RollingHashUpdated` confirming anchoring
3. Restart coordinator (coordinated with parallel test)
4. Send 5 more L1→L2 messages after restart
5. Wait for `RollingHashUpdated` for post-restart messages
6. Assert: Rolling hash and anchored message number match expected values

---

### l2.spec.ts — Layer 2 Transaction Handling

Tests L2-specific transaction validation and execution.

**Test: Reject Oversized Calldata**
1. Generate random bytes exceeding `TRANSACTION_CALLDATA_LIMIT` (30,000 bytes)
2. Attempt to call `DummyContract.setPayload()` with explicit gasLimit (bypasses estimateGas)
3. Assert: Transaction reverts with "Calldata of transaction is greater than the allowed max"

**Test: Accept Valid Calldata Size**
1. Generate 1000 random bytes (under limit)
2. Use `linea_estimateGas` to get proper gas parameters
3. Submit transaction
4. Assert: Transaction succeeds with `receipt.status = 1`

**Test: Legacy Transaction (Type 0)**
1. Build type-0 transaction with gasPrice
2. Submit and wait for receipt
3. Assert: Transaction included successfully

**Test: EIP-1559 Transaction (Type 2)**
1. Build type-2 transaction with maxFeePerGas and maxPriorityFeePerGas
2. Use `linea_estimateGas` for fee estimation
3. Assert: Transaction included successfully

**Test: Access List Transaction (Type 1)**
1. Build type-1 transaction with accessList
2. Submit with both empty and populated access lists
3. Assert: Both variants succeed

---

### linea-besu-fleet.spec.ts — Node Consistency

Tests that leader and follower nodes return consistent results.

**Test: Leader/Follower Response Matching**
1. Wait until `currentL2BlockNumber` on L1 is > 1 (finalization occurred)
2. Call `linea_estimateGas` on leader and follower with identical parameters
3. Assert: `maxPriorityFeePerGas`, `maxFeePerGas` match exactly
4. Call `eth_estimateGas` on both nodes
5. Assert: Gas estimates match
6. Query `"finalized"` block from both nodes
7. Assert: Block number and hash match
8. Query `"latest"` block from both nodes
9. Assert: Block number and hash match

---

### liveness.spec.ts — Sequencer Uptime Detection

Tests the liveness feed that tracks sequencer downtime.

**Test: Liveness Transactions After Restart**
1. Stop sequencer via `docker stop sequencer`
2. Wait 9 seconds (longer than `liveness-max-block-age` threshold)
3. Record last block timestamp and number
4. Restart sequencer via `docker restart sequencer`
5. Poll for `AnswerUpdated` events on `LineaSequencerUptimeFeed` contract
6. Assert: First event at index 0 indicates downtime (status=1, timestamp=lastBlockTimestamp)
7. Assert: Second event at index 1 indicates uptime (status=0, timestamp > lastBlockTimestamp)

---

### send-bundle.spec.ts — Transaction Bundling

Tests the `linea_sendBundle` RPC method for atomic transaction inclusion.

**Test: Bundle Inclusion**
1. Generate 3 signed transactions from same sender with sequential nonces
2. Call `linea_sendBundle` with target block number = current + 5
3. Poll until target block number is reached
4. Assert: All 3 transactions have `receipt.status = 1`

**Test: Bundle Rejection (Invalid Transactions)**
1. Generate 3 transactions each sending 5 ETH from account with only 10 ETH total
2. Submit bundle (second tx will fail due to insufficient balance)
3. Wait for target block
4. Assert: None of the bundled transactions were included

**Test: Bundle Cancellation**
1. Submit bundle targeting block current + 10
2. Wait until current + 5, then call `linea_cancelBundle` with same UUID
3. Wait for target block
4. Assert: Bundled transactions were not included

---

### transaction-exclusion.spec.ts — Rejection Tracking

Tests the transaction exclusion API that tracks rejected transactions.

**Test: Get Rejection Status from RPC**
1. Build transaction that triggers traces module limit overflow
2. Attempt to send (expect rejection with "is above the limit")
3. Compute transaction hash from unsigned transaction
4. Poll `linea_getTransactionExclusionStatusV1` with the hash
5. Assert: Response contains `txHash`, `txRejectionStage = "RPC"`, correct `from` address

---

### shomei-get-proof.spec.ts — Merkle Proof Verification

Tests Shomei state proof generation and verification.

**Test: Valid Proof from Shomei Frontend**
1. Wait until `currentL2BlockNumber` on L1 is > 1
2. Call `linea_getProof` on Shomei frontend for a known address
3. Call `rollup_getZkEVMStateMerkleProofV0` on Shomei backend to get `zkEndStateRootHash`
4. Call `SparseMerkleProof.verifyProof()` with proof nodes, leaf index, and state root
5. Assert: Verification returns true
6. Modify state root hash (flip last hex digit)
7. Call `verifyProof()` with modified root
8. Assert: Verification returns false

---

### opcodes.spec.ts — EVM Opcode Support

Tests that all EVM opcodes execute correctly on L2.

**Test: Estimate Gas for Opcode Execution**
1. Call `linea_estimateGas` targeting `OpcodeTester.executeAllOpcodes()`
2. Assert: Returns valid `maxPriorityFeePerGas`, `maxFeePerGas`, `gasLimit` > 0

**Test: Execute All Opcodes**
1. Read `rollingBlockDetailComputations` value before execution
2. Call `OpcodeTester.executeAllOpcodes()` with explicit gas limit
3. Read value after execution
4. Assert: Value changed (proves opcodes executed and modified state)

## Directory Structure

```
e2e/
├── src/
│   ├── bridge-tokens.spec.ts
│   ├── messaging.spec.ts
│   ├── submission-finalization.spec.ts
│   ├── restart.spec.ts
│   ├── linea-besu-fleet.spec.ts
│   ├── liveness.spec.ts
│   ├── l2.spec.ts
│   ├── opcodes.spec.ts
│   ├── send-bundle.spec.ts
│   ├── transaction-exclusion.spec.ts
│   ├── shomei-get-proof.spec.ts
│   │
│   ├── common/
│   │   ├── utils.ts              # Shared utilities
│   │   ├── constants.ts          # Test constants
│   │   ├── deployments.ts        # Contract deployments
│   │   ├── generateL2Traffic.ts  # Traffic generation
│   │   └── types.ts              # Type definitions
│   │
│   └── config/
│       ├── jest/
│       │   ├── global-setup.ts   # Test setup
│       │   ├── global-teardown.ts # Test cleanup
│       │   └── setup.ts
│       ├── logger/               # Logging utilities
│       └── tests-config/
│           ├── environments/
│           │   ├── local.ts      # Local stack config
│           │   ├── dev.ts        # Dev environment
│           │   └── sepolia.ts    # Testnet
│           └── accounts/         # Account management
│
├── jest.config.ts
└── package.json
```

## Test Infrastructure

### Prerequisites

```bash
# Start full stack
make start-env-with-tracing-v2-ci

# Verify services
docker compose -f docker/compose-tracing-v2-ci-extension.yml ps
```

### Configuration

```typescript
// config/local.ts
export const config = {
  l1: {
    rpcUrl: 'http://localhost:8445',
    chainId: 31648428,
  },
  l2: {
    rpcUrl: 'http://localhost:8545',
    chainId: 1337,
  },
  contracts: {
    lineaRollup: '0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9',
    l2MessageService: '0xe537D669CA013d86EBeF1D64e40fC74CADC91987',
    tokenBridge: {
      l1: '0x...',
      l2: '0x...',
    },
  },
  accounts: {
    deployer: {
      privateKey: '0xac0974bec...',
    },
  },
};
```

### Global Setup

```typescript
// global-setup.ts
export default async function globalSetup() {
  // Deploy test contracts
  await deployTestContracts();
  
  // Start background L2 traffic
  await startTrafficGeneration();
  
  console.log('E2E setup complete');
}

async function deployTestContracts() {
  const deployer = new Wallet(config.accounts.deployer.privateKey);
  
  // Deploy DummyContract
  const DummyContract = await ethers.getContractFactory('DummyContract');
  await DummyContract.connect(deployer).deploy();
  
  // Deploy TestContract
  const TestContract = await ethers.getContractFactory('TestContract');
  await TestContract.connect(deployer).deploy();
  
  // Deploy LineaSequencerUptimeFeed
  // ...
}
```

## Test Examples

### Bridge Tokens Test

```typescript
// bridge-tokens.spec.ts
describe('Bridge Tokens', () => {
  describe('L1 → L2', () => {
    it('should bridge ERC20 from L1 to L2', async () => {
      const { l1Wallet, l2Wallet } = await generateAccounts();
      
      // Mint test tokens on L1
      await testToken.connect(l1Wallet).mint(
        l1Wallet.address,
        parseEther('100')
      );
      
      // Approve token bridge
      await testToken.connect(l1Wallet).approve(
        tokenBridge.address,
        parseEther('10')
      );
      
      // Bridge tokens
      const tx = await tokenBridge.connect(l1Wallet).bridgeToken(
        testToken.address,
        parseEther('10'),
        l2Wallet.address
      );
      
      // Wait for anchoring
      await waitForEvents(
        l2MessageService,
        'RollingHashUpdated',
        1
      );
      
      // Claim on L2
      const message = await getMessageFromTx(tx);
      await l2MessageService.connect(l2Wallet).claimMessage(
        message.from,
        message.to,
        message.fee,
        message.value,
        l2Wallet.address,
        message.calldata,
        message.nonce
      );
      
      // Verify balance
      const bridgedToken = await getBridgedTokenAddress(testToken.address);
      const balance = await IERC20(bridgedToken).balanceOf(l2Wallet.address);
      expect(balance).to.equal(parseEther('10'));
    });
  });
});
```

### Submission & Finalization Test

```typescript
// submission-finalization.spec.ts
describe('Submission and Finalization', () => {
  it('should submit and finalize blocks', async () => {
    // Generate L2 transactions
    await generateL2Transactions(10);
    
    // Wait for data submission
    const submissionEvent = await waitForEvents(
      lineaRollup,
      'DataSubmittedV3',
      1,
      { timeout: 120000 }
    );
    
    expect(submissionEvent.shnarf).to.not.be.null;
    
    // Wait for finalization
    const finalizationEvent = await waitForEvents(
      lineaRollup,
      'DataFinalizedV3',
      1,
      { timeout: 300000 }
    );
    
    expect(finalizationEvent.endBlockNumber).to.be.gt(0);
    
    // Verify safe block updated
    const safeBlock = await l2Provider.getBlock('safe');
    expect(safeBlock.number).to.be.gte(finalizationEvent.endBlockNumber);
  });
});
```

### Coordinator Restart Test

```typescript
// restart.spec.ts
describe('Coordinator Restart', () => {
  it('should resume blob submission after restart', async () => {
    // Get initial finalized block
    const initialFinalized = await lineaRollup.currentL2BlockNumber();
    
    // Restart coordinator
    await execDockerCommand('docker restart coordinator');
    
    // Wait for coordinator to be healthy
    await waitForHealthy('coordinator');
    
    // Generate more transactions
    await generateL2Transactions(10);
    
    // Verify blob submission resumes
    const newFinalized = await awaitUntil(
      async () => lineaRollup.currentL2BlockNumber(),
      (blockNum) => blockNum > initialFinalized,
      { timeout: 300000, interval: 5000 }
    );
    
    expect(newFinalized).to.be.gt(initialFinalized);
  });
});
```

### Fleet Consistency Test

```typescript
// linea-besu-fleet.spec.ts
describe('Fleet Consistency', () => {
  it('should have consistent linea_estimateGas across nodes', async () => {
    const tx = {
      from: account.address,
      to: testContract.address,
      data: testContract.interface.encodeFunctionData('doSomething'),
    };
    
    // Get estimate from leader
    const leaderEstimate = await leaderClient.estimateGas(tx);
    
    // Get estimate from follower
    const followerEstimate = await followerClient.estimateGas(tx);
    
    expect(leaderEstimate).to.equal(followerEstimate);
  });
  
  it('should have matching finalized blocks', async () => {
    const leaderBlock = await leaderProvider.getBlock('finalized');
    const followerBlock = await followerProvider.getBlock('finalized');
    
    expect(leaderBlock.hash).to.equal(followerBlock.hash);
  });
});
```

## Running Tests

```bash
cd e2e

# Install dependencies (generates TypeChain types)
pnpm install

# Run all tests (except fleet and liveness)
pnpm run test:e2e:local

# Run specific test file
pnpm run test:e2e:local -- bridge-tokens.spec.ts

# Run fleet tests
pnpm run test:e2e:fleet:local

# Run liveness tests
pnpm run test:e2e:liveness:local
```

## Jest Configuration

```typescript
// jest.config.ts
export default {
  preset: 'ts-jest',
  testEnvironment: 'node',
  testTimeout: 180000,          // 3 minutes
  maxConcurrency: 7,
  maxWorkers: '75%',
  globalSetup: './src/global-setup.ts',
  globalTeardown: './src/global-teardown.ts',
  testMatch: ['**/*.spec.ts'],
  testPathIgnorePatterns: [
    'linea-besu-fleet.spec.ts',
    'liveness.spec.ts',
  ],
};
```

## Utility Functions

### Wait for Events

```typescript
// utils/events.ts
export async function waitForEvents<T>(
  contract: Contract,
  eventName: string,
  count: number,
  options: { timeout?: number } = {}
): Promise<T[]> {
  const { timeout = 60000 } = options;
  const events: T[] = [];
  
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error(`Timeout waiting for ${count} ${eventName} events`));
    }, timeout);
    
    contract.on(eventName, (...args) => {
      events.push(args as unknown as T);
      if (events.length >= count) {
        clearTimeout(timer);
        contract.removeAllListeners(eventName);
        resolve(events);
      }
    });
  });
}
```

### Await Until

```typescript
// utils/common.ts
export async function awaitUntil<T>(
  fn: () => Promise<T>,
  predicate: (value: T) => boolean,
  options: { timeout?: number; interval?: number } = {}
): Promise<T> {
  const { timeout = 60000, interval = 1000 } = options;
  const startTime = Date.now();
  
  while (Date.now() - startTime < timeout) {
    const value = await fn();
    if (predicate(value)) {
      return value;
    }
    await sleep(interval);
  }
  
  throw new Error('Timeout in awaitUntil');
}
```

## Dependencies

- **Jest**: Test framework
- **ethers**: Ethereum interactions
- **TypeChain**: Contract type generation
- **Docker**: Container management (restart, exec)

# Smart Contracts

> Solidity contracts for the Linea rollup, messaging, and bridging protocols.

> **Diagram:** [Contracts Architecture](../diagrams/contracts-architecture.mmd) (Mermaid source)

## Overview

The contracts directory contains all Solidity smart contracts for:
- L1 Rollup (submission, finalization, verification)
- Cross-chain messaging (L1 ↔ L2)
- Token bridging (ERC20 bridging)
- Security and governance

## Contract Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                            L1 (ETHEREUM)                               │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                         LineaRollup                              │  │
│  │                                                                  │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐   │  │
│  │  │ ZkEvmV2     │  │ L1Message   │  │ LineaRollupYield        │   │  │
│  │  │             │  │  Service    │  │  Extension              │   │  │
│  │  │ - Submit    │  │             │  │                         │   │  │
│  │  │   blobs     │  │ - Send msg  │  │ - Native ETH yield      │   │  │
│  │  │ - Finalize  │  │ - Anchor    │  │                         │   │  │
│  │  │   blocks    │  │   roots     │  │                         │   │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────┘   │  │
│  │                                                                  │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐   │  │
│  │  │ Eip4844Blob │  │ LivenessRe  │  │ ClaimMessageV1          │   │  │
│  │  │  Acceptor   │  │  covery     │  │                         │   │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────┘   │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                           │                                            │
│  ┌────────────────────────▼─────────────────────────────────────────┐  │
│  │                      PlonkVerifier                               │  │
│  │                                                                  │  │
│  │  Verifies aggregated ZK proofs on-chain                          │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                     TokenBridge (L1)                             │  │
│  │                                                                  │  │
│  │  - Bridge ERC20 tokens to L2                                     │  │
│  │  - Claim tokens bridged from L2                                  │  │
│  │  - Deploy BridgedToken contracts                                 │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────┐
│                             L2 (LINEA)                                 │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                     L2MessageService                             │  │
│  │                                                                  │  │
│  │  - Send messages to L1                                           │  │
│  │  - Anchor L1→L2 message hashes                                   │  │
│  │  - Claim messages from L1                                        │  │
│  │  - Rolling hash for L2→L1 Merkle tree                            │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                     TokenBridge (L2)                             │  │
│  │                                                                  │  │
│  │  - Bridge tokens to L1                                           │  │
│  │  - Claim tokens bridged from L1                                  │  │
│  │  - Mint/burn bridged token representations                       │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                      Predeploy Contracts                         │  │
│  │                                                                  │  │
│  │  - WithdrawalQueue                                               │  │
│  │  - ConsolidationQueue                                            │  │
│  │  - BeaconChainDeposit                                            │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
contracts/
├── src/
│   ├── rollup/                 # L1 Rollup contracts
│   │   ├── LineaRollup.sol     # Main rollup contract
│   │   ├── LineaRollupBase.sol # Base rollup logic
│   │   └── ZkEvmV2.sol         # ZK verification logic
│   │
│   ├── messaging/              # Message service contracts
│   │   ├── l1/
│   │   │   └── L1MessageService.sol
│   │   └── l2/
│   │       └── L2MessageService.sol
│   │
│   ├── bridging/               # Token bridge contracts
│   │   └── token/
│   │       ├── TokenBridge.sol
│   │       ├── TokenBridgeBase.sol
│   │       └── BridgedToken.sol
│   │
│   ├── verifiers/              # ZK verifier contracts
│   │   ├── PlonkVerifierMainnetFull.sol
│   │   ├── PlonkVerifierSepoliaFull.sol
│   │   └── PlonkVerifierForDataAggregation.sol
│   │
│   ├── security/               # Security utilities
│   │   ├── access/             # Access control
│   │   ├── limiting/           # Rate limiting
│   │   ├── pausing/            # Pause functionality
│   │   └── reentrancy/         # Reentrancy protection
│   │
│   ├── governance/             # Governance contracts
│   │   └── TimeLock.sol
│   │
│   ├── yield/                  # Native yield contracts
│   │   ├── YieldManager.sol
│   │   ├── interfaces/         # Yield interfaces
│   │   └── libs/               # Yield libraries
│   │
│   ├── recovery/               # Recovery utilities
│   │   └── RecoverFunds.sol
│   │
│   ├── proxies/                # Proxy contracts
│   │   └── CallForwardingProxy.sol
│   │
│   ├── operational/            # Operational contracts
│   │   ├── RollupRevenueVault.sol
│   │   ├── L1LineaTokenBurner.sol
│   │   └── LineaSequencerUptimeFeed.sol
│   │
│   └── predeploy/              # L2 system contracts
│       ├── UpgradeableWithdrawalQueuePredeploy.sol
│       ├── UpgradeableConsolidationQueuePredeploy.sol
│       └── UpgradeableBeaconChainDepositPredeploy.sol
│
├── test/
│   └── hardhat/                # TypeScript tests
│       ├── rollup/
│       ├── messaging/
│       ├── bridging/
│       └── security/
│
├── deploy/                     # Deployment scripts
│   ├── 01_deploy_PlonkVerifier.ts
│   ├── 02_deploy_Timelock.ts
│   ├── 03_deploy_LineaRollup.ts
│   ├── 04_deploy_L2MessageService.ts
│   ├── 05_deploy_BridgedToken.ts
│   └── 06_deploy_TokenBridge.ts
│
└── local-deployments-artifacts/ # Local deployment scripts
    ├── deployPlonkVerifierAndLineaRollupV6.ts
    ├── deployL2MessageServiceV1.ts
    └── deployBridgedTokenAndTokenBridgeV1_1.ts
```

## Core Contracts

### LineaRollup (L1)

The main L1 contract managing state submissions and finalization.

**Key Functions:**

```solidity
// Submit blob data via EIP-4844
function submitBlobs(
    BlobSubmission[] calldata _blobSubmissions,
    bytes32 _parentShnarf,
    bytes32 _finalShnarf
) external;

// Finalize state with ZK proof
function finalizeBlocks(
    bytes calldata _aggregatedProof,
    uint256 _proofType,
    FinalizationDataV3 calldata _finalizationData
) external;

// Send message to L2
function sendMessage(
    address _to,
    uint256 _fee,
    bytes calldata _calldata
) external payable;

// Claim message from L2 (requires Merkle proof from Shomei)
function claimMessageWithProof(
    ClaimMessageWithProofParams calldata _params
) external;

// ClaimMessageWithProofParams struct:
// - bytes32[] proof      - Merkle proof from L2 state
// - uint256 messageNumber - Message nonce
// - uint32 leafIndex     - Leaf index in Merkle tree
// - address from         - Original sender on L2
// - address to           - Recipient on L1
// - uint256 fee          - Fee for claiming
// - uint256 value        - ETH value to transfer
// - address feeRecipient - Address to receive the fee
// - bytes32 merkleRoot   - L2 Merkle root (anchored on L1)
// - bytes data           - Calldata to execute
```

**Events:**

```solidity
event DataSubmittedV3(
    bytes32 parentShnarf,
    bytes32 indexed shnarf,
    bytes32 finalStateRootHash
);

event DataFinalizedV3(
    uint256 indexed startBlockNumber,
    uint256 indexed endBlockNumber,
    bytes32 indexed shnarf,
    bytes32 parentStateRootHash,
    bytes32 finalStateRootHash
);

event MessageSent(
    address indexed _from,
    address indexed _to,
    uint256 _fee,
    uint256 _value,
    uint256 _nonce,
    bytes _calldata,
    bytes32 indexed _messageHash
);
```

### L2MessageService (L2)

L2 message service for cross-chain communication.

**Key Functions:**

```solidity
// Send message to L1
function sendMessage(
    address _to,
    uint256 _fee,
    bytes calldata _calldata
) external payable;

// Claim message from L1
function claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    address payable _feeRecipient,
    bytes calldata _calldata,
    uint256 _nonce
) external;

// Anchor L1 message hashes (called by L1_L2_MESSAGE_SETTER)
function anchorL1L2MessageHashes(
    bytes32[] calldata _messageHashes,
    uint256 _startingMessageNumber,
    uint256 _finalMessageNumber
) external;
```

### TokenBridge (L1 & L2)

ERC20 token bridging with beacon proxy pattern.

**Key Functions:**

```solidity
// Bridge token to other chain
function bridgeToken(
    address _token,
    uint256 _amount,
    address _recipient
) external payable;

// Bridge token with permit
function bridgeTokenWithPermit(
    address _token,
    uint256 _amount,
    address _recipient,
    PermitData calldata _permitData
) external payable;

// Complete bridging (called via message service)
function completeBridging(
    address _nativeToken,
    uint256 _amount,
    address _recipient,
    uint256 _chainId,
    bytes calldata _tokenMetadata
) external;
```

### PlonkVerifier

On-chain ZK proof verification.

```solidity
interface IPlonkVerifier {
    function verify(
        bytes calldata _proof,
        uint256[] calldata _publicInputs
    ) external view returns (bool);
}
```

## Deployment

### Local Development

```bash
# Deploy all contracts
make deploy-contracts

# Deploy L1 rollup only
make deploy-linea-rollup-v6

# Deploy L2 message service
make deploy-l2messageservice

# Deploy token bridge (L1)
make deploy-token-bridge-l1

# Deploy token bridge (L2)
make deploy-token-bridge-l2
```

### Environment Variables

```bash
# L1 Deployment
PRIVATE_KEY=0x...
RPC_URL=http://localhost:8445
VERIFIER_CONTRACT_NAME=IntegrationTestTrueVerifier
LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH=0x...
LINEA_ROLLUP_SECURITY_COUNCIL=0x...
LINEA_ROLLUP_OPERATORS=0x...,0x...

# L2 Deployment
PRIVATE_KEY=0x...
RPC_URL=http://localhost:8545
L2MSGSERVICE_SECURITY_COUNCIL=0x...
L2MSGSERVICE_L1L2_MESSAGE_SETTER=0x...
```

## Testing

```bash
cd contracts

# Compile
npx hardhat compile

# Run all tests
npx hardhat test

# Run specific test file
npx hardhat test test/hardhat/rollup/LineaRollup.ts

# Coverage
npx hardhat coverage
```

## Security Features

### Access Control

```solidity
// Role-based access
bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");
bytes32 public constant VERIFIER_SETTER_ROLE = keccak256("VERIFIER_SETTER_ROLE");
bytes32 public constant RATE_LIMIT_SETTER_ROLE = keccak256("RATE_LIMIT_SETTER_ROLE");
```

### Pause Functionality

```solidity
// Pausable operations
enum PauseType {
    GENERAL,
    L1_L2_MESSAGING,
    L2_L1_MESSAGING,
    BLOB_SUBMISSION,
    FINALIZATION
}

function pauseByType(PauseType _pauseType) external;
function unpauseByType(PauseType _pauseType) external;
```

### Rate Limiting

```solidity
// Limit value transfer per period
uint256 public periodInSeconds = 86400; // 24 hours
uint256 public limitInWei;

function _addUsedAmount(uint256 _usedAmount) internal;
function _resetAmountUsedInPeriod() internal;
```

## Upgrade Pattern

All major contracts use OpenZeppelin's **Transparent Upgradeable Proxy** pattern:

```
┌──────────────────────────────────────────────────────────────────────┐
│                    TRANSPARENT PROXY PATTERN                         │
│                                                                      │
│  ┌──────────────────┐    ┌──────────────────┐    ┌───────────────┐  │
│  │ TransparentProxy │───▶│   ProxyAdmin     │───▶│   TimeLock    │  │
│  │                  │    │                  │    │               │  │
│  │  - delegatecall  │    │  - upgrade()     │    │  - schedule() │  │
│  │  - fallback()    │    │  - changeAdmin() │    │  - execute()  │  │
│  └────────┬─────────┘    └──────────────────┘    └───────────────┘  │
│           │                                                          │
│           │  delegatecall                                            │
│           ▼                                                          │
│  ┌──────────────────┐                                                │
│  │  Implementation  │                                                │
│  │  (LineaRollup)   │                                                │
│  │                  │                                                │
│  │  - Business logic│                                                │
│  │  - No upgrade fn │                                                │
│  └──────────────────┘                                                │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Key characteristics:**
- **Admin separation**: Only the ProxyAdmin can call upgrade functions; regular users interact with the implementation
- **No upgrade logic in implementation**: Unlike UUPS, the upgrade logic lives in the proxy itself
- **TimeLock governance**: Upgrades are protected by a timelock for security

## Contract Addresses

### Mainnet

| Contract | Address |
|----------|---------|
| LineaRollup | `0xd19d4B5d358258f05D7B411E21A1460D11B0876F` |
| L2MessageService | `0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec` |
| TokenBridge (L1) | `0x051F1D88f0aF5763fB888eC78378F1109b52Cd01` |
| TokenBridge (L2) | `0x353012dc4a9A6cF55c941bADC267f82004A8ceB9` |

### Sepolia (Testnet)

| Contract | Address |
|----------|---------|
| LineaRollup | `0xb218f8a4bc926cf1ca7b3423c154a0d627bdb7e5` |
| L2MessageService | `0x971e727e956690b9957be6d51ec16e73acac83a7` |

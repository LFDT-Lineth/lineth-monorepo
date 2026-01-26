# LineaRollup V7 Upgrade Verification - Sepolia

This configuration verifies the LineaRollup V7 upgrade deployed on Sepolia.

## Transaction Details

- **TX Hash**: [0xc3d0a93b353c78cfbe1d9e450c8683087fa8835723a40690a4e694a269f51e9c](https://sepolia.etherscan.io/tx/0xc3d0a93b353c78cfbe1d9e450c8683087fa8835723a40690a4e694a269f51e9c)
- **Date**: Jan 20, 2026
- **Block**: 8,008,468

## Key Addresses

| Contract | Address |
|----------|---------|
| Proxy | `0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48` |
| Implementation | `0xCaAa421FfCF701bEFd676a2F5d0A161CCFA5a07E` |
| Safe (Multisig) | `0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1` |
| YieldManager | `0xafeB487DD3E3Cb0342e8CF0215987FfDc9b72c9b` |

---

## Contract: LineaRollup-Proxy

```verifier
name: LineaRollup-Proxy
address: 0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48
chain: ethereum-sepolia
artifact: ../../../../contracts/deployments/bytecode/2026-01-14/LineaRollup.json
isProxy: true
ozVersion: v4
schema: ../schemas/linea-rollup.json
```

### State Verification

The following checks verify the state after `reinitializeLineaRollupV7()` execution:

| Type | Description | Check | Params | Expected |
|------|-------------|-------|--------|----------|
| viewCall | Contract version | `CONTRACT_VERSION` | | `7.0` |
| viewCall | OPERATOR_ROLE → Safe | `hasRole` | `0x76ef52a5344b10ed112c1d48c7c06f51e919518ea6fb005f9b25b359b955e3be`,`0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1` | true |
| viewCall | VERIFIER_SETTER_ROLE → Safe | `hasRole` | `0x220bd22ef7c53d75fe3eac0a09e90815a0c5ba4f9e8da8b039542cd3db347258`,`0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1` | true |
| viewCall | VERIFIER_SETTER_ROLE → Dead | `hasRole` | `0x220bd22ef7c53d75fe3eac0a09e90815a0c5ba4f9e8da8b039542cd3db347258`,`0x000000000000000000000000000000000000dEaD` | true |
| viewCall | PAUSE_NATIVE_YIELD_STAKING_ROLE | `hasRole` | `0xcc10d6eec3c757d645e27b3f3001a3ba52f692da0bce25fabf58c6ecaf376450`,`0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1` | true |
| viewCall | UNPAUSE_NATIVE_YIELD_STAKING_ROLE | `hasRole` | `0x4b4665d8754e6ea0608430ef3e91c1b45c72aafe8800e289cd35f38d85361858`,`0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1` | true |
| slot | OZ _initialized version | `0x0` | uint8 | `7` |
| storagePath | Yield manager address | `LineaRollupYieldExtensionStorage:_yieldManager` | | `0xafeB487DD3E3Cb0342e8CF0215987FfDc9b72c9b` |

### Role Reference

| Role Name | keccak256 Hash |
|-----------|----------------|
| `OPERATOR_ROLE` | `0x76ef52a5344b10ed112c1d48c7c06f51e919518ea6fb005f9b25b359b955e3be` |
| `VERIFIER_SETTER_ROLE` | `0x220bd22ef7c53d75fe3eac0a09e90815a0c5ba4f9e8da8b039542cd3db347258` |
| `PAUSE_NATIVE_YIELD_STAKING_ROLE` | `0xcc10d6eec3c757d645e27b3f3001a3ba52f692da0bce25fabf58c6ecaf376450` |
| `UNPAUSE_NATIVE_YIELD_STAKING_ROLE` | `0x4b4665d8754e6ea0608430ef3e91c1b45c72aafe8800e289cd35f38d85361858` |

---

## Contract: LineaRollup-Implementation

```verifier
name: LineaRollup-Implementation
address: 0xCaAa421FfCF701bEFd676a2F5d0A161CCFA5a07E
chain: ethereum-sepolia
artifact: ../../../../contracts/deployments/bytecode/2026-01-14/LineaRollup.json
isProxy: false
```

No state verification needed for implementation contract (bytecode and ABI verification only).

---

## Usage

```bash
# Set RPC URL
export ETHEREUM_SEPOLIA_RPC_URL="https://sepolia.infura.io/v3/YOUR_KEY"

# Run verification using the ethers CLI
cd contract-integrity-verifier/verifier-ethers
pnpm build
node dist/cli.mjs -c ../verifier-core/examples/configs/sepolia-linea-rollup-v7.config.md -v
```

## Expected Results

- **Bytecode**: PASS (implementation matches artifact)
- **ABI**: PASS (all selectors present)
- **State**: PASS (all 8 checks should pass)

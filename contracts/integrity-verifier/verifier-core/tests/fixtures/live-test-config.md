# Live Integration Test Configuration

This configuration is used for live integration tests against Sepolia.
Requires `ETHEREUM_SEPOLIA_RPC_URL` environment variable.

## Contract: LineaRollup-Proxy

```verifier
name: LineaRollup-Proxy
address: 0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48
chain: ethereum-sepolia
artifact: ../../../../../contracts/deployments/bytecode/2026-01-14/LineaRollup.json
isProxy: true
ozVersion: v4
```

### State Verification

| Type | Description | Check | Params | Expected |
|------|-------------|-------|--------|----------|
| viewCall | Contract version | `CONTRACT_VERSION` | | `7.0` |
| slot | Initialized version | `0x0` | uint8 | `7` |

---

## Contract: LineaRollup-Implementation

```verifier
name: LineaRollup-Implementation
address: 0xCaAa421FfCF701bEFd676a2F5d0A161CCFA5a07E
chain: ethereum-sepolia
artifact: ../../../../../contracts/deployments/bytecode/2026-01-14/LineaRollup.json
isProxy: false
```

No state verification needed for implementation contract - bytecode comparison only.

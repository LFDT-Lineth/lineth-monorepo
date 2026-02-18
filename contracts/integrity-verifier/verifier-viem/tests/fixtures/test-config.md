# Test Configuration for Integration Tests

This configuration uses mock artifacts for testing the contract integrity verifier.

## Contract: TestYieldManager-Proxy

```verifier
name: TestYieldManager-Proxy
address: 0x73bF00aD18c7c0871EBA03Bcbef8C98225f9CEaA
chain: test-chain
artifact: ./artifacts/YieldManager.json
isProxy: true
ozVersion: v4
schema: ../../examples/schemas/yield-manager.json
```

### State Verification

| Type | Description | Check | Params | Expected |
|------|-------------|-------|--------|----------|
| viewCall | DEFAULT_ADMIN_ROLE check | `hasRole` | `0x0000000000000000000000000000000000000000000000000000000000000000`,`0xaC83Ca663356e8C8cca7AdA5a60C01f98c383430` | true |
| viewCall | L1 Message Service address | `L1_MESSAGE_SERVICE` | | `0x24B0E20c3Cec999C8A6723FCfC06d5c88fB4a056` |
| slot | Initialized version | `0x0` | uint8 | `1` |
| storagePath | Target withdrawal reserve | `YieldManagerStorage:targetWithdrawalReservePercentageBps` | | `8000` |

---

## Contract: TestLineaRollup-Proxy

```verifier
name: TestLineaRollup-Proxy
address: 0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48
chain: test-chain
artifact: ./artifacts/LineaRollup.json
isProxy: true
ozVersion: v4
schema: ../../examples/schemas/linea-rollup.json
```

### State Verification

| Type | Description | Check | Params | Expected |
|------|-------------|-------|--------|----------|
| viewCall | Contract version | `CONTRACT_VERSION` | | `7.0` |
| viewCall | OPERATOR_ROLE check | `hasRole` | `0x76ef52a5344b10ed112c1d48c7c06f51e919518ea6fb005f9b25b359b955e3be`,`0xe6Ec44e651B6d961c15f1A8df9eA7DFaDb986eA1` | true |
| slot | Initialized version | `0x0` | uint8 | `7` |
| storagePath | Yield manager address | `LineaRollupYieldExtensionStorage:_yieldManager` | | `0xafeB487DD3E3Cb0342e8CF0215987FfDc9b72c9b` |

---

## Contract: TestLineaRollup-Implementation

```verifier
name: TestLineaRollup-Implementation
address: 0xCaAa421FfCF701bEFd676a2F5d0A161CCFA5a07E
chain: test-chain
artifact: ./artifacts/LineaRollup.json
isProxy: false
```

No state verification needed for implementation contract.

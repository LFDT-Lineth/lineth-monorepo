# Security audit reports

This page is the canonical inventory of published security audit reports for Lineth components,
Linea Mainnet contracts, and relevant dependencies.

## Browse by component

- [Prover, proving libraries, and verifier contracts](#prover-proving-libraries-and-verifier-contracts)
- [Cryptographic primitives](#cryptographic-primitives)
- [Rollup, message service, and token bridge](#rollup-message-service-and-token-bridge)
- [Feature-specific contracts](#feature-specific-contracts)
- [Third-party dependency audits](#third-party-dependency-audits)

## Prover, proving libraries, and verifier contracts

| Date | Component or change | Auditor | Report |
| --- | --- | --- | --- |
| August 2025 | Limitless Prover | Least Authority | [Report](https://github.com/LFDT-Lineth/lineth-monorepo/blob/4d5e19c4161acfba12fcabc7e9db4a624f39690b/docs/audits/2025%20August%20-%20Least%20Authority%20-%20Linea%20zkEVM%20Limitless%20Prover%20Final%20Audit%20Report.pdf) |
| November 2024 | Linea zkEVM | Least Authority | [Report](https://github.com/Consensys/gnark/blob/d9cbd38409edb5cfdbf9daa9dba841ad432feff9/audits/2024-11%20-%20Least%20Authority%20-%20Linea%20zkEVM.pdf) |
| September 2024 | gnark and GKR | Least Authority | [Report](https://github.com/Consensys/gnark/blob/53ce9b74e7ab372aa7358f1eac7fe3d689743f5f/audits/2024-09%20-%20Least%20Authority%20-%20arithm%20and%20GKR.pdf) |
| June 2024 | gnark PLONK Prover and verifier | OpenZeppelin | [Report](https://blog.openzeppelin.com/linea-prover-audit) |
| May 2024 | gnark standard library | ZKSecurity | [Report](https://github.com/Consensys/gnark/blob/53ce9b74e7ab372aa7358f1eac7fe3d689743f5f/audits/2024-05%20-%20zksecurity%20-%20gnark%20std.pdf) |
| November 2023 | gnark PLONK Solidity verifier template | OpenZeppelin | [Report](https://blog.openzeppelin.com/linea-verifier-audit-1) |
| June 2023 | gnark PLONK Solidity verifier | Consensys Diligence | [Report](https://consensys.io/diligence/audits/2023/06/linea-plonk-verifier/) |

## Cryptographic primitives

| Date | Component or change | Auditor | Report |
| --- | --- | --- | --- |
| January 2026 | Linea Poseidon2 | Consensys Diligence | [Report](https://diligence.security/audits/2026/01/linea-poseidon2/) |
| June 2024 | Wizard cryptography and mathematics | ZKSecurity | [Report](https://reports.zksecurity.xyz/reports/consensys-wizard-crypto-math/) |

## Rollup, message service, and token bridge

| Round | Component or change | Auditor | Report |
| --- | --- | --- | --- |
| Eighth | Forced transactions | Consensys Diligence | [Report](https://diligence.security/audits/2026/02/linea-forced-transactions/) |
| Eighth | Forced transactions | Cyfrin | [Report](https://github.com/Cyfrin/cyfrin-audit-reports/blob/9397aab67a15854b08eb302722cdebf5ddb3ac3f/reports/2026-06-18-cyfrin-linea-forced-txns-v2.0.pdf) |
| Seventh | Modularization, pause cooldown, and dynamic chain configuration | Consensys Diligence | [Report](https://diligence.security/audits/2026/02/linea-rollup-update/) |
| Seventh | Modularization, pause cooldown, and dynamic chain configuration | Cyfrin | [Report](https://github.com/Cyfrin/cyfrin-audit-reports/blob/37c82603d92e097c60811ba63ebb6484574be56f/reports/2026-03-27-cyfrin-linea-mixed-upgrade-v2.0.pdf) |
| Sixth | Yield Manager | Consensys Diligence | [Report](https://diligence.security/audits/2025/12/linea-yield-manager/) |
| Sixth | Yield Manager | OpenZeppelin | [Report](https://www.openzeppelin.com/news/linea-yield-manager-audit) |
| Sixth | Yield Manager | Cyfrin | [Report](https://github.com/Cyfrin/cyfrin-audit-reports/blob/d0f4523388a964891a17cb04c6b3cc26da8c788a/reports/2026-02-12-cyfrin-linea-yield-manager-v2.0.pdf) |
| Fifth | Granular role updates | Consensys Diligence | [Report](https://diligence.consensys.io/audits/2024/12/linea-rollup-update/) |
| Fifth | Granular role updates | OpenZeppelin | [Report](https://blog.openzeppelin.com/linearollup-and-tokenbridge-role-upgrade) |
| Fifth | Granular role updates | Cyfrin | [Report](https://github.com/Cyfrin/cyfrin-audit-reports/blob/642b409c207d0e31679467480c3d9b8797b98696/reports/2025-01-06-cyfrin-linea-v2.2.pdf) |
| Fourth | Differential review since the second audit round | Consensys Diligence | [Report](https://consensys.io/diligence/audits/2024/07/linea-rollup-update/) |
| Fourth | Gas optimizations | OpenZeppelin | [Report](https://blog.openzeppelin.com/linea-gas-optimizations-audit) |
| Fourth | Full codebase, gas optimizations, and TokenBridge updates | Cyfrin | [Report](https://github.com/Cyfrin/cyfrin-audit-reports/blob/main/reports/2024-05-24-cyfrin-linea-v2.0.pdf) |
| Third | Blob submission | OpenZeppelin | [Report](https://blog.openzeppelin.com/linea-blob-submission-audit) |
| Second | Proof aggregation, data compression, and message service updates | Consensys Diligence | [Report](https://consensys.io/diligence/audits/2024/01/linea-contracts-update/) |
| Second | Proof aggregation, data compression, and message service updates | OpenZeppelin | [Report](https://blog.openzeppelin.com/linea-v2-audit) |
| First | PLONK verifier | Consensys Diligence | [Report](https://consensys.io/diligence/audits/2023/06/linea-plonk-verifier/) |
| First | Message Service and rollup | Consensys Diligence | [Report](https://consensys.io/diligence/audits/2023/06/linea-message-service/) |
| First | Canonical Token Bridge | Consensys Diligence | [Report](https://consensys.io/diligence/audits/2023/06/linea-canonical-token-bridge/) |
| First | Linea Bridge | OpenZeppelin | [Report](https://blog.openzeppelin.com/linea-bridge-audit-1) |
| First | Linea Verifier | OpenZeppelin | [Report](https://blog.openzeppelin.com/linea-verifier-audit-1) |

## Feature-specific contracts

| Component or change | Auditor | Report |
| --- | --- | --- |
| Burn mechanism | Consensys Diligence | [Report](https://diligence.consensys.io/audits/2025/10/linea-burn-mechanism/) |
| Burn mechanism | OpenZeppelin | [Report](https://www.openzeppelin.com/news/linea-burn-mechanism-audit) |
| Burn mechanism | Cyfrin | [Report](https://github.com/Cyfrin/cyfrin-audit-reports/blob/2e780ec0c4c8401ff97a7e0cd509ece52ab35cb3/reports/2025-11-03-cyfrin-linea-burn-v2.2.pdf) |
| Token and airdrop contracts | Consensys Diligence | [Report](https://diligence.consensys.io/audits/2025/07/linea-token-and-airdrop-contracts/) |
| Token generation event | OpenZeppelin | [Report](https://blog.openzeppelin.com/linea-tge-audit) |
| Linea token contracts | Cyfrin | [Report](https://github.com/Cyfrin/cyfrin-audit-reports/blob/b9aace5911e3ff84488cb5199cfd28e7fe24d6aa/reports/2025-09-10-cyfrin-linea-tokens-v2.5.pdf) |

## Third-party dependency audits

These reports cover dependencies used by the technology stack and were commissioned by other
organizations.

| Date | Dependency | Auditor | Commissioned by | Report |
| --- | --- | --- | --- | --- |
| August 2023 | gnark Groth16 Solidity verifier template | Least Authority | Worldcoin | [Report](https://leastauthority.com/wp-content/uploads/2023/08/Worldcoin_Groth16_Verifier_in_EVM_Smart_Contract_Final_Audit_Report.pdf) |
| May 2023 | gnark-crypto KZG | Sigma Prime | Ethereum Foundation | [Report](https://github.com/Consensys/gnark/blob/53ce9b74e7ab372aa7358f1eac7fe3d689743f5f/audits/2024-05%20-%20Sigma%20Prime%20-%20kzg.pdf) |
| October 2022 | gnark-crypto | Kudelski Security | Algorand Foundation | [Report](https://github.com/Consensys/gnark/blob/53ce9b74e7ab372aa7358f1eac7fe3d689743f5f/audits/2022-10%20-%20Kudelski%20-%20gnark-crypto.pdf) |
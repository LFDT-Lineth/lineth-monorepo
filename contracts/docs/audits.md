# Audits
## Fourth Audit Round (Latest)
**Diligence**
- Differential audit since second audit round: https://consensys.io/diligence/audits/2024/07/linea-rollup-update/

**Open Zeppelin**
- Gas optimization audit: https://blog.openzeppelin.com/linea-gas-optimizations-audit

**Cyfrin**
- Full codebase audit including gas optimizations and TokenBridge updates: https://github.com/Cyfrin/cyfrin-audit-reports/blob/main/reports/2024-05-24-cyfrin-linea-v2.0.pdf 

## Third Audit Round
**Open Zeppelin**

- Blob submission audit: https://blog.openzeppelin.com/linea-blob-submission-audit

## Second Audit Round

**Diligence**
- Proof aggregation, data compression and message service updates Audit: https://consensys.io/diligence/audits/2024/01/linea-contracts-update/

**Open Zeppelin**

- Proof aggregation, data compression and message service updates Audit: https://blog.openzeppelin.com/linea-v2-audit

## First Audit Round

**Diligence**

- Plonk Verifier: https://consensys.io/diligence/audits/2023/06/linea-plonk-verifier/
- Message Service & Rollup: https://consensys.io/diligence/audits/2023/06/linea-message-service/
- Canonical Token Bridge: https://consensys.io/diligence/audits/2023/06/linea-canonical-token-bridge/

**Open Zeppelin**

- Linea Bridge Audit: https://blog.openzeppelin.com/linea-bridge-audit-1
- Linea Verifier Audit: https://blog.openzeppelin.com/linea-verifier-audit-1



---

## Installation and testing

To run the solution's tests, coverage and gas reporting, be sure to install pnpm and then
```
# Install all the dependencies

pnpm install

pnpm run test

pnpm run test:reportgas

pnpm run coverage
```
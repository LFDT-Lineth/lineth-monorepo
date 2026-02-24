# E2E Test Agent Guidelines

## Using Contract ABIs and Bytecode

Never inline ABI or bytecode in test files. Use the generated typed exports instead.

Artifacts live in `contracts/local-deployments-artifacts/`:
- `static-artifacts/` - non-upgradeable contracts
- `dynamic-artifacts/` - upgradeable contracts

To add a new contract to E2E tests:

1. Add the contract name to `INCLUDE_FILES` in `e2e/scripts/generateAbi.ts`.
   The generator runs automatically via `postinstall` during `pnpm i`, producing `e2e/src/generated/<ContractName>Abi.ts`.
2. Import in your test:
   ```ts
   import { MyContractAbi, MyContractAbiBytecode } from "./generated";
   ```

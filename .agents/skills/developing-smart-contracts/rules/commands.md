# Commands

## Linting

Run linting before committing:

```bash
pnpm run lint:fix
```

---

### Linting Commands

```bash
# Solidity linting
pnpm -F contracts run lint:sol

# TypeScript linting
pnpm -F contracts run lint:ts

# Fix all lint issues
pnpm -F contracts run lint:fix
```

### Test Commands

```bash
# Run tests
cd contracts && pnpm hardhat test

# Run tests with coverage
cd contracts && SOLIDITY_COVERAGE=true pnpm hardhat coverage

# Run specific test file
cd contracts && pnpm hardhat test test/hardhat/messaging/l1/L1MessageService.ts
```
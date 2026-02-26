# AGENTS.md — bridge-ui

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

Next.js 16 frontend application for the Linea token bridge. Uses React 19, Wagmi/Viem for wallet interactions, Zustand for state management, and Playwright for testing.

## How to Run

```bash
# Dev server
pnpm -F bridge-ui dev

# Build (standalone output)
pnpm -F bridge-ui run build

# Start production server
pnpm -F bridge-ui run start

# Type check
pnpm -F bridge-ui run check-types

# Lint
pnpm -F bridge-ui run lint
pnpm -F bridge-ui run lint:fix

# Unit tests
pnpm -F bridge-ui run test:unit

# E2E tests (requires Playwright browsers)
pnpm -F bridge-ui run install:playwright
pnpm -F bridge-ui run test:e2e:headless

# E2E with UI mode
pnpm -F bridge-ui run test:e2e:headless:ui

# Build Synpress cache (Metamask fixture)
pnpm -F bridge-ui run build:cache
```

## Frontend-Specific Conventions

### Framework

- Next.js 16.1.5 with standalone output mode
- React 19.1.5
- Turbopack for dev server
- SVGR for SVG imports
- SASS for styling

### State and Data

- **State management:** Zustand 4.5.4
- **Wallet connection:** Wagmi 3.4.1 + Viem 2.45.0
- **Async data:** TanStack React Query 5
- **Validation:** Zod 3.24.2
- **Animations:** GSAP + Motion

### ESLint

Uses `@consensys/eslint-config/nextjs` export from the shared config.

### Environment Variables

- Templates: `.env.template`, `.env.test`, `.env.production`
- Required variables include API keys for CoinMarketCap, WalletConnect, Web3Auth, LayerSwap, LiFi
- Changes must be reflected in `.env.template`

### Remote Images

Allowed domains configured in `next.config.ts`: CoinMarketCap, CoinGecko, Linea, Contentful CDN.

## Testing

- **Unit tests:** Playwright with `UNIT=true` flag — `test:unit`
- **E2E tests:** Playwright with Synpress for Metamask interactions — `test:e2e:headless`
- **Browser:** Chromium project only
- **CI workflow:** `.github/workflows/bridge-ui-e2e-tests.yml` triggers on `bridge-ui/**` changes

## Agent Rules (Overrides)

- Run `pnpm -F bridge-ui run check-types` after TypeScript changes
- Test environment variables must use placeholders, not real API keys
- SVG assets are imported as React components via SVGR
- The `@consensys/linea-sdk-viem` workspace package provides bridge SDK functionality

# Bugbot Review Instructions

Automated security and code quality enforcement across the Linea monorepo.
For the complete repository guide, see [AGENTS.md](../AGENTS.md).

## Precedence and Discovery

- Canonical repository instructions: [AGENTS.md](../AGENTS.md)
- Cursor docs index: [rules/documentation.mdc](rules/documentation.mdc)
- If this file conflicts with `../AGENTS.md`, follow `../AGENTS.md`.

## Scope

Review all pull requests targeting `main`. Focus on files changed in the PR diff. Skip generated files (`typechain-types/`, `dist/`, `build/`, `coverage/`, `node_modules/`).

## Rule Sets

### Security Logging

Prevents leaking secrets, credentials, and sensitive data in logs, error messages, tests, and config files. Applies to all code files (`.ts`, `.js`, `.kt`, `.java`, `.go`, `.sol`, `.py`).

- [Rules](rules/security-logging-guidelines/RULES.md)
- [Examples](rules/security-logging-guidelines/EXAMPLES.md)

### API and Asset Versioning

Enforces backwards compatibility for public APIs and versioned assets (ABIs, configs). Components released beyond devnet must create new versioned methods/assets instead of breaking existing ones. Applies to all code files plus `.abi` and `.json`.

- [Rules](rules/versioning-guidelines/RULES.md)
- [Examples](rules/versioning-guidelines/EXAMPLES.md)

## Red Flags (Always Comment)

- Secrets, API keys, private keys, or credentials in source code, logs, or config
- Missing NatSpec on public/external Solidity functions, events, or errors
- Breaking changes to public APIs without a new versioned method/asset
- Wildcard Solidity imports (`import "path/to/File.sol"`)
- `console.log` or `console.error` with sensitive data (tokens, keys, connection strings)
- New dependencies added without justification in the PR description
- Environment variable changes not reflected in `.env.template` or `.env.example`
- Hardcoded addresses or chain IDs that should be configurable
- Unchecked arithmetic in Solidity without a safety comment
- Missing error handling on async operations (unhandled promise rejections, uncaught exceptions)
- Type assertions (`as any`, `as unknown as X`) that bypass type safety

## Checklists by Change Type

### Smart Contracts (`contracts/**/*.sol`)

- [ ] NatSpec: `@notice`, `@param` (in order), `@return` on all public/external items
- [ ] Named imports only
- [ ] License header: `Apache-2.0` for interfaces, `AGPL-3.0` for contracts
- [ ] Pragma: exact `0.8.33` for contracts, caret `^0.8.33` for interfaces/abstract/libraries
- [ ] Custom errors preferred over revert strings
- [ ] Storage layout: no slot collisions in upgradeable contracts
- [ ] `reinitializer(N)` pattern (never `initializer`)
- [ ] Access control on state-changing functions
- [ ] Events emitted for all state changes
- [ ] Reentrancy guards on functions that make external calls
- [ ] Zero-value checks via `ErrorUtils`
- [ ] Gas: `calldata` for read-only inputs, cached storage reads

### TypeScript/JavaScript (`**/*.ts`, `**/*.js`)

- [ ] No `any` types unless unavoidable with a comment explaining why
- [ ] Error handling on all async operations
- [ ] Secrets loaded from environment variables, never hardcoded
- [ ] Tests added or updated for changed logic
- [ ] ESLint and Prettier compliance

### Kotlin/Java (`**/*.kt`, `**/*.java`)

- [ ] `toString()` overrides on config classes redact secrets, and data classes with ByteArray fields
- [ ] `equals()/hashCode()` overrides on data classes with ByteArray fields
- [ ] Structured logging with logfmt format, without sensitive data
- [ ] Spotless formatting compliance
- [ ] Tests added or updated for changed logic
- [ ] Integration tests use Docker test containers where applicable

### Go (`prover/**/*.go`)

- [ ] `gofmt` compliant
- [ ] `golangci-lint` passes
- [ ] Error returns checked (no ignored errors)
- [ ] Build tags preserved (`nocorset`, `fuzzlight`)

### Frontend (`bridge-ui/**`)

- [ ] No API keys or secrets in client-side code
- [ ] Environment variable changes reflected in `.env.template`
- [ ] Accessibility: semantic HTML, ARIA labels where needed
- [ ] No bundle size regressions from large new dependencies

### CI/CD and Config (`.github/**`, `docker/**`, `config/**`)

- [ ] Workflow syntax valid (YAML)
- [ ] No secrets exposed in workflow logs or outputs
- [ ] Path filters updated if new directories are added
- [ ] Docker images use specific tags, not `latest`

## False Positive Handling

- Ignore files matching: `**/dist/**`, `**/build/**`, `**/generated/**`, `**/node_modules/**`, `**/typechain-types/**`, `**/coverage/**`, `**/*.d.ts`
- Hardhat default private keys (`0xac0974bec...`) in local deployment scripts are acceptable — they are well-known test keys
- Generated ABI JSON files do not need manual review

## Review Style

- Short, direct comments on the specific file and line
- Propose a concrete fix direction (code snippet or pattern reference)
- Classify severity: **blocking** (must fix), **important** (should fix), **minor** (nice to have)
- One comment per distinct issue
- Reference rule files for detailed guidance (e.g., "See `.cursor/rules/security-logging-guidelines/RULES.md`")

## Enforcement

When a violation is detected, **block the change** with a clear explanation and suggest a safe alternative. Always reference the relevant rule's examples for correct patterns.

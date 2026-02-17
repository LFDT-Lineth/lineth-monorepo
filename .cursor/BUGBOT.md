# BUGBOT Rules

Automated security and code quality enforcement across the Linea monorepo.

## Rule Sets

### Security Logging

Prevents leaking secrets, credentials, and sensitive data in logs, error messages, tests, and config files. Applies to all code files (`.ts`, `.js`, `.kt`, `.java`, `.go`, `.sol`, `.py`).

- [Rules](rules/security-logging-guidelines/RULES.md)
- [Examples](rules/security-logging-guidelines/EXAMPLES.md)

### API and Asset Versioning

Enforces backwards compatibility for public APIs and versioned assets (ABIs, configs). Components released beyond devnet must create new versioned methods/assets instead of breaking existing ones. Applies to all code files plus `.abi` and `.json`.

- [Rules](rules/versioning-guidelines/RULES.md)
- [Examples](rules/versioning-guidelines/EXAMPLES.md)

## Enforcement

When a violation is detected, **block the change** with a clear explanation and suggest a safe alternative. Always reference the relevant rule's examples for correct patterns.

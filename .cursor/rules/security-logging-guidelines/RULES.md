---
description: Security-Sensitive Information Logging Prevention - Rules
globs: "**/*.ts, **/*.js, **/*.kt, **/*.java, **/*.go, **/*.sol, **/*.py"
alwaysApply: true
---

# Security Logging Rules

Refer to [EXAMPLES.md](EXAMPLES.md) for detailed examples of what to do and what not to do.

## MUST NOT

- Log passwords, API keys, auth tokens, session IDs, private keys (cryptographic/SSH/PGP), JWT tokens, AWS keys, or GitHub tokens.
- Log URLs containing sensitive query parameters (`token`, `apiKey`, `secret`, `password`, `auth`) without sanitization.
- Log environment variables wholesale or config objects without redacting sensitive fields.
- Log RPC URLs with embedded tokens or database connection strings containing credentials.
- Log private keys or raw signatures during transaction signing or message signing.
- Expose secrets in error messages, stack traces, exception propagation, metrics labels, trace data, or health-check endpoints.
- Hardcode secrets in source code, operation scripts, configuration files, documentation, or code comments.
- Include real credentials in tests, fixtures, mock servers, Postman collections, or example code.
- Log request/response bodies or headers that contain auth tokens.

## MUST

- Sanitize URLs by redacting sensitive query parameters before any logging.
- Redact sensitive fields when logging objects or configuration; override `toString()` on config classes (Kotlin/Java) to redact secrets.
- Configure structured logging frameworks (Log4j/SLF4J/etc.) to filter sensitive data.
- Load secrets from environment variables or secure vaults exclusively.
- Use obviously fake/placeholder credentials in all tests, fixtures, `.env.template` files, and documentation.
- Ensure `.env` files are in `.gitignore` and template files reference placeholders only.
- Sanitize error details before propagation - log `error.message` and `error.code`, not full config or request objects.

## MAY log

- Log levels, categories, timestamps, and request IDs.
- Endpoint paths without query parameters.
- Error types and status codes.
- Configuration keys (not values); indicate presence with `[CONFIGURED]` / `[NOT SET]`.
- Wallet addresses and transaction hashes (never private keys).
- Partial identifiers (last 4 characters).

## Enforcement

When a violation is detected, **block the change** with a clear explanation and suggest a safe alternative. Always reference [EXAMPLES.md](EXAMPLES.md) for correct patterns.

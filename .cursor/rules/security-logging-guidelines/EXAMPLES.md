---
description: Security-Sensitive Information Logging Prevention - Examples
globs: **/*.ts, **/*.js, **/*.kt, **/*.java, **/*.go, **/*.sol, **/*.py
alwaysApply: true
---

# Security Logging -- WRONG / CORRECT Examples

## Authentication & Credentials

```typescript
// WRONG
console.log(`API Key: ${apiKey}`);
logger.info(`Using key: ${process.env.SECRET_KEY}`);
console.error(`Login failed for ${username} with password ${password}`);
logger.debug(`Auth attempt: ${JSON.stringify({ user, password })}`);
console.log(`Bearer token: ${authToken}`);

// CORRECT
console.log('API key configured successfully');
logger.debug(`Auth attempt for user: ${username} [credentials redacted]`);
logger.info(`Using key ending in: ...${apiKey.slice(-4)}`);
```

## URLs with Sensitive Parameters

```typescript
// WRONG
console.log(`Calling API: https://api.example.com/data?token=${token}`);
logger.info(`Request URL: ${fullUrl}`);

// CORRECT
function sanitizeUrl(url: string): string {
  const urlObj = new URL(url);
  ['token', 'apiKey', 'api_key', 'secret', 'password', 'auth'].forEach(param => {
    if (urlObj.searchParams.has(param)) {
      urlObj.searchParams.set(param, '[REDACTED]');
    }
  });
  return urlObj.toString();
}

console.log(`Calling API: ${sanitizeUrl(fullUrl)}`);
logger.info(`Request to endpoint: ${new URL(fullUrl).pathname}`);
```

## Environment Variables & Configuration

```typescript
// WRONG
console.log('Environment:', process.env);
logger.debug(`Config: ${JSON.stringify(config)}`);
console.log(`DB connection: ${dbConnectionString}`);

// CORRECT
function sanitizeConnectionString(connStr: string): string {
  return connStr.replace(/(:\/\/[^:]+:)[^@]+(@)/, '$1[REDACTED]$2');
}
logger.info(`Database: ${sanitizeConnectionString(dbConnectionString)}`);
```

## User Privacy Data

```typescript
// CORRECT
logger.info(`User action: ${hashUserId(userId)}`);
logger.debug(`Email domain: ${email.split('@')[1]}`);
logger.info(`Payment method ending in: ${cardNumber.slice(-4)}`);
```

## Testing with Sensitive Data

```typescript
// WRONG
const testApiKey = 'sk-real-api-key-12345';

// CORRECT
const testApiKey = 'test-mock-key-' + randomString();
const mockToken = 'mock-jwt-token-for-testing';
process.env.API_KEY = 'test-key-not-real';
```

## Exception Handling

```typescript
// WRONG
catch (error) {
  console.error('Request failed:', error, requestConfig);
}

// CORRECT
catch (error) {
  const safeError = {
    message: error.message,
    code: error.code,
    endpoint: sanitizeUrl(error.config?.url)
  };
  logger.error('Request failed:', safeError);
}
```

## Kotlin/Java (Coordinator Services)

```kotlin
// WRONG
log.info("Starting coordinator with config: $config")
log.debug("API key: ${apiKey}")

// CORRECT
log.info("Starting coordinator service")
log.info("Configuration loaded")
```

```kotlin
// WRONG - Config class exposes secrets via toString()
data class CoordinatorConfig(val apiKey: String, val secretKey: String)

// CORRECT - Override toString() to redact
data class CoordinatorConfig(val apiKey: String, val secretKey: String) {
    override fun toString(): String {
        return "CoordinatorConfig(apiKey=[REDACTED], secretKey=[REDACTED])"
    }
}
```

```kotlin
// WRONG
log.info("Database connection: $dbConnectionString")

// CORRECT
fun sanitizeConnectionString(connStr: String): String {
    return connStr.replace(Regex("://([^:]+):([^@]+)@"), "://$1:[REDACTED]@")
}
log.info("Database connection: ${sanitizeConnectionString(dbConnectionString)}")
```

```kotlin
// WRONG
log.info("Making request to: $fullUrl")
log.debug("Request headers: $headers")

// CORRECT
fun sanitizeUrl(url: String): String {
    return url.replace(Regex("[?&](token|apiKey|api_key|secret|password|auth)=[^&]*"), "")
}
log.info("Making request to: ${sanitizeUrl(fullUrl)}")
```

## Blockchain / Web3

```typescript
// WRONG
console.log('Signing transaction with key:', privateKey);
console.log('Full wallet:', wallet);

// CORRECT
console.log(`From address: ${wallet.address}`);
console.log(`Transaction hash: ${txHash}`);
```

```typescript
// WRONG
console.log(`Deploying with private key: ${process.env.DEPLOYER_PRIVATE_KEY}`);
console.log(`Using API key: ${apiKey}`);

// CORRECT
console.log('Deployer account configured');
console.log(`Deploying from address: ${deployerAddress}`);
```

```typescript
// WRONG
console.log(`Connecting to: ${rpcUrlWithToken}`);
console.log(`API endpoint: https://api.infura.io/v3/${projectId}`);

// CORRECT
function sanitizeRpcUrl(url: string): string {
  return url.replace(/\/v3\/[a-zA-Z0-9]+/, '/v3/[REDACTED]');
}
console.log(`Connecting to: ${sanitizeRpcUrl(rpcUrl)}`);
```

```typescript
// WRONG
console.log('Initializing provider:', providerConfig);

// CORRECT
const safeConfig = {
  network: config.network,
  chainId: config.chainId,
  rpcUrl: config.rpcUrl ? '[CONFIGURED]' : '[NOT SET]',
  apiKey: config.apiKey ? '[CONFIGURED]' : '[NOT SET]',
};
console.log('Deployment config:', JSON.stringify(safeConfig));
```

## API Testing / Postman

```json
// WRONG - Real credentials in environment file
{
  "API_KEY": "sk-real-api-key-abc123",
  "AUTH_TOKEN": "real-bearer-token-xyz"
}

// CORRECT - Placeholder values
{
  "API_KEY": "{{YOUR_API_KEY}}",
  "AUTH_TOKEN": "{{YOUR_AUTH_TOKEN}}"
}
```

```typescript
// WRONG
console.log('Making request:', { url: fullUrl, headers: headers });
console.log('Response:', response);

// CORRECT
console.log('Making request to:', sanitizeUrl(fullUrl));
console.log('Response status:', response.status);
```

```typescript
// WRONG
const PRIVATE_KEY = '0x1234...real-private-key';
const API_ENDPOINT = 'https://api.linea.build?token=real-token-abc123';

// CORRECT
const TEST_PRIVATE_KEY = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';
const API_ENDPOINT = process.env.TEST_API_ENDPOINT || 'http://localhost:8545';
```

## Detection Regex Patterns

Use these to detect potential secrets in logging statements:

```regex
# API Keys & Tokens
(api[_-]?key|apikey|access[_-]?token|auth[_-]?token|bearer[_-]?token)["\s:=]+[a-zA-Z0-9_\-]{20,}

# AWS Credentials
AKIA[0-9A-Z]{16}
aws_secret_access_key["\s:=]+[a-zA-Z0-9/+=]{40}

# GitHub Tokens
gh[pousr]_[a-zA-Z0-9]{36}

# Generic Secrets
(password|passwd|pwd|secret)["\s:=]+\S+

# Private Keys
-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----

# JWT Tokens
eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+

# URLs with sensitive params
(https?:\/\/[^\s]*[?&](token|key|secret|password|auth)=[^\s&]+)
```

## Pre-commit Hook

```bash
git diff --cached | grep -E "(password|api_key|secret|token)['\"]?\s*[:=]"
```

## Recommended Sanitization Libraries

- TypeScript/JavaScript: `redact-object`, `fast-redact`
- Java/Kotlin: `logback-mask`, custom filters
- Go: `zap` with custom encoders
- Python: `logging` with custom formatters

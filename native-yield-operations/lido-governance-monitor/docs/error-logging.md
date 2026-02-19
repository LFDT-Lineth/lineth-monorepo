# Error Logging & Severity Classification

This document describes the error logging architecture and severity classification rules for the Lido Governance Monitor service.

## Overview

The service uses structured logging with severity levels to enable log-based alerting. Logs are aggregated externally (Loki/CloudWatch), and alerts are configured to trigger on counts of specific severity levels.

## Design Decisions

### Why Log-Based Metrics?

The Lido Governance Monitor runs as a **Kubernetes CronJob**. Traditional Prometheus counters/gauges don't work well because Prometheus marks metrics as stale after ~5 minutes of no scrapes. Since the pod terminates after each run, metrics disappear.

### Severity via Metadata Field

We use Winston's standard log levels (`error`, `warn`, `info`, `debug`) with an additional `severity` metadata field:

```
level=ERROR severity=CRITICAL message="Database connection failed"
level=ERROR severity=ERROR message="Validation failed"
level=WARN severity=WARN message="AI analysis failed, will retry"
```

**Why not a custom Winston level?**
- Winston's `colorize()` controls ANSI colors in ArgoCD logs
- Adding custom levels requires modifying shared library
- Metadata field approach is simpler and equally queryable

## Severity Levels

### CRITICAL

**Definition**: External dependency failures that require immediate investigation.

**When to use**:
- HTTP 4xx/5xx responses from external APIs
- Database connection or query failures
- Slack webhook failures (main alert channel)
- Network timeouts or connection errors
- Any failure in communication with external systems

**Examples**:
- Discourse API returns 500 Internal Server Error
- PostgreSQL connection refused
- Anthropic API rate limited (429)
- Slack webhook returns 403 Forbidden

### ERROR

**Definition**: Processing failures not caused by external dependencies.

**When to use**:
- Input validation failures (Zod schema)
- Output validation failures (AI response schema)
- JSON parse errors
- Missing required data (data integrity issues)
- Unexpected response formats

**Examples**:
- AI response missing `riskScore` field
- Invalid proposal URL format
- JSON.parse() throws on malformed response
- Proposal missing assessment data

**Alert action**: Review logs, may indicate prompt/schema drift.

### WARN

**Definition**: Non-blocking issues worth noting but not actionable immediately.

**When to use**:
- Scheduled retries (will be attempted again)
- Transient conditions that self-resolve

**Examples**:
- AI analysis failed, will retry next run
- Individual proposal fetch failed, continuing with others

**Alert action**: Monitor trends, investigate if sustained.

## Component-Specific Classification

### DiscourseClient
| Scenario | Severity |
|----------|----------|
| HTTP 4xx/5xx response | CRITICAL |
| Schema validation failure | ERROR |
| Network exception | CRITICAL |

### ClaudeAIClient
| Scenario | Severity |
|----------|----------|
| API request exception (timeout, 4xx, 5xx) | CRITICAL |
| Input validation error | ERROR |
| Output schema validation error | ERROR |
| JSON parse error | ERROR |
| Missing text content in response | ERROR |

### SlackClient
| Scenario | Severity |
|----------|----------|
| Main alert webhook HTTP error | CRITICAL |
| Main alert network exception | CRITICAL |
| Audit webhook HTTP error | CRITICAL |
| Audit webhook network exception | CRITICAL |

### Services (Poller, Processor, Notification)
| Scenario | Severity |
|----------|----------|
| Top-level catch (entire pipeline failed) | CRITICAL |
| DB operation failure | CRITICAL |
| Normalization failure | ERROR |
| Individual proposal processing failure | WARN |
| Data integrity issue (missing assessment) | ERROR |

## Usage Example

```typescript
// Factory creates per-service logger
const logger = createLidoGovernanceMonitorLogger("DiscourseClient");

// External dependency failure
try {
  const response = await fetch(url);
  if (!response.ok) {
    logger.critical("Failed to fetch proposals", {
      status: response.status,
      statusText: response.statusText,
    });
    return undefined;
  }
} catch (error) {
  logger.critical("Network error fetching proposals", { error });
  return undefined;
}

// Processing failure
const result = schema.safeParse(data);
if (!result.success) {
  logger.error("Schema validation failed", {
    errors: result.error.errors,
  });
  return undefined;
}

// Audit failure - still critical but non-blocking
if (!auditResult.success) {
  logger.critical("Audit log failed, continuing", {
    error: auditResult.error,
  });
}
```

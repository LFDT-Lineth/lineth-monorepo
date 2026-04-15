# Prompt Eval

## What

Detects prompt regressions in the risk assessment system prompt by comparing AI outputs against human-labeled ground truth.

## Strategy

Binary classification against a fixed corpus of 20 real Lido Discourse proposals. Each proposal is labeled by a human reviewer as `shouldAlert: true/false` (i.e. "would this reach the Slack alert threshold?"). The eval measures TP/TN/FP/FN against those labels.

Alert threshold: `effectiveRisk = round((riskScore * confidence) / 100) >= 60` (configurable).

Single run per proposal - LLM variance is accepted; borderline cases are visible in the raw scores.

## Implementation

| File | Role |
|------|------|
| `scripts/run-eval.ts` | Entry point - loads fixtures, calls `ClaudeAIClient.analyzeProposal()`, prints CLI table |
| `scripts/eval/proposals.jsonl` | Fixed corpus (20 real Discourse proposals) |
| `scripts/eval/golden-set.json` | Human labels keyed by `sourceId` |

The script calls `ClaudeAIClient` directly - no DB, no state machine. Exits 0 regardless of results (informational, not a gate).

## How to Conduct

### Prerequisites

- `ANTHROPIC_API_KEY` set in env
- Golden set labels reviewed and updated in `scripts/eval/golden-set.json` before running

### Run

```bash
# From native-yield-operations/lido-governance-monitor/
ANTHROPIC_API_KEY=sk-ant-xxx pnpm exec tsx scripts/run-eval.ts
```

With overrides:

```bash
ANTHROPIC_API_KEY=sk-ant-xxx \
RISK_THRESHOLD=50 \
CLAUDE_MODEL=claude-sonnet-4-20250514 \
pnpm exec tsx scripts/run-eval.ts
```

### Interpret Output

```
+-----------+----------------------------------------------+------+------------+-----------------+-----------+-----------------+
| SourceId  | Title                                        | Risk | Confidence | Effective Risk  | AI Alert? | Result          |
+-----------+----------------------------------------------+------+------------+-----------------+-----------+-----------------+
| 16        | Lido Improvement Proposal Process            |    5 |         90 |               5 | NO        | True Negative   |
+-----------+----------------------------------------------+------+------------+-----------------+-----------+-----------------+

Threshold: 60 | Model: claude-sonnet-4-20250514 | Proposals: 20

  Correct:          18/20 (90.0%)
  False Positives:  2
  False Negatives:  1
  AI Failures:      0
```

- **False Positives** - AI alarmed; human said no alert. Noisy prompt.
- **False Negatives** - AI missed; human said alert. Dangerous prompt regression.
- **AI Failures** - `analyzeProposal()` returned `undefined` (API error or schema validation failure).

### Updating the Golden Set

When adding new proposals to `scripts/eval/proposals.jsonl`, add a corresponding entry to `scripts/eval/golden-set.json`. Read the proposal text against `src/prompts/risk-assessment-system.md` to decide the label.

/**
 * Prompt eval script for the Lido Governance Monitor risk assessment.
 *
 * Runs ClaudeAIClient.analyzeProposal() against a fixed corpus of proposals
 * and compares results to human-labeled ground truth, printing a CLI table.
 *
 * Usage:
 * ANTHROPIC_API_KEY=sk-ant-xxx \
 * pnpm --filter @consensys/lido-governance-monitor exec tsx scripts/run-eval.ts
 *
 * Optional env vars:
 * RISK_THRESHOLD=60                          # effectiveRisk threshold for alerting (default: 60)
 * CLAUDE_MODEL=claude-sonnet-4-20250514      # Model to use (default: claude-sonnet-4-20250514)
 * ANTHROPIC_MAX_OUTPUT_TOKENS=4096           # Max output tokens (default: 4096)
 * ANTHROPIC_MAX_PROPOSAL_CHARS=700000        # Max proposal chars sent to AI (default: 700000)
 */

import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

import Anthropic from "@anthropic-ai/sdk";

import { ClaudeAIClient } from "../src/clients/ClaudeAIClient.js";
import { Assessment } from "../src/core/entities/Assessment.js";
import { ILidoGovernanceMonitorLogger } from "../src/utils/logging/index.js";

interface ProposalFixture {
  source: string;
  sourceId: string;
  url: string;
  title: string;
  author: string;
  sourceCreatedAt: string;
  rawProposalText: string;
}

interface GoldenLabel {
  shouldAlert: boolean;
}

type GoldenSet = Record<string, GoldenLabel>;

type EvalResult = {
  sourceId: string;
  title: string;
  humanShouldAlert: boolean;
} & (
  | {
      status: "success";
      riskScore: number;
      confidence: number;
      effectiveRisk: number;
      aiAlerts: boolean;
    }
  | {
      status: "ai_failure";
    }
);

const noopLogger: ILidoGovernanceMonitorLogger = {
  name: "eval",
  info: () => {},
  error: () => {},
  warn: () => {},
  debug: () => {},
  critical: () => {},
};

function mapSourceToProposalType(source: string): "discourse" | "snapshot" | "onchain_vote" {
  switch (source) {
    case "DISCOURSE":
      return "discourse";
    case "SNAPSHOT":
      return "snapshot";
    case "LDO_VOTING_CONTRACT":
    case "STETH_VOTING_CONTRACT":
      return "onchain_vote";
    default:
      return "discourse";
  }
}

function loadProposals(filePath: string): ProposalFixture[] {
  const content = readFileSync(filePath, "utf-8");
  return content
    .split("\n")
    .filter((line) => line.trim().length > 0)
    .map((line) => JSON.parse(line) as ProposalFixture);
}

function loadGoldenSet(filePath: string): GoldenSet {
  const content = readFileSync(filePath, "utf-8");
  return JSON.parse(content) as GoldenSet;
}

function classifyResult(aiAlerts: boolean, humanShouldAlert: boolean): string {
  if (aiAlerts && humanShouldAlert) return "True Positive";
  if (!aiAlerts && !humanShouldAlert) return "True Negative";
  if (aiAlerts && !humanShouldAlert) return "False Positive";
  return "False Negative";
}

function printTable(results: EvalResult[], threshold: number, model: string): void {
  const header = [
    "SourceId".padEnd(10),
    "Title".padEnd(44),
    "Risk".padStart(6),
    "Confidence".padStart(12),
    "Effective Risk".padStart(16),
    "AI Alert?".padEnd(11),
    "Result".padEnd(17),
  ];
  const separator = header.map((h) => "-".repeat(h.length));

  console.log("+" + separator.map((s) => "-" + s + "-").join("+") + "+");
  console.log("|" + header.map((h) => " " + h + " ").join("|") + "|");
  console.log("+" + separator.map((s) => "-" + s + "-").join("+") + "+");

  for (const r of results) {
    const truncTitle = r.title.length > 44 ? r.title.substring(0, 41) + "..." : r.title.padEnd(44);

    if (r.status === "ai_failure") {
      const row = [
        r.sourceId.padEnd(10),
        truncTitle,
        "-".padStart(6),
        "-".padStart(12),
        "-".padStart(16),
        "N/A".padEnd(11),
        "AI Failure".padEnd(17),
      ];
      console.log("|" + row.map((c) => " " + c + " ").join("|") + "|");
    } else {
      const result = classifyResult(r.aiAlerts, r.humanShouldAlert);
      const row = [
        r.sourceId.padEnd(10),
        truncTitle,
        String(r.riskScore).padStart(6),
        String(r.confidence).padStart(12),
        String(r.effectiveRisk).padStart(16),
        (r.aiAlerts ? "YES" : "NO").padEnd(11),
        result.padEnd(17),
      ];
      console.log("|" + row.map((c) => " " + c + " ").join("|") + "|");
    }
  }

  console.log("+" + separator.map((s) => "-" + s + "-").join("+") + "+");

  const analyzed = results.length;
  const failures = results.filter((r) => r.status === "ai_failure").length;
  const successful = results.filter((r) => r.status === "success") as Extract<EvalResult, { status: "success" }>[];
  const fp = successful.filter((r) => r.aiAlerts && !r.humanShouldAlert).length;
  const fn = successful.filter((r) => !r.aiAlerts && r.humanShouldAlert).length;
  const correct = successful.length - fp - fn;

  console.log("");
  console.log(`Threshold: ${threshold} | Model: ${model} | Proposals: ${analyzed}`);
  console.log("");
  console.log(
    `  Correct:          ${correct}/${successful.length} (${successful.length > 0 ? ((correct / successful.length) * 100).toFixed(1) : "0.0"}%)`,
  );
  console.log(`  False Positives:  ${fp}`);
  console.log(`  False Negatives:  ${fn}`);
  console.log(`  AI Failures:      ${failures}`);
}

async function main(): Promise<void> {
  const apiKey = process.env.ANTHROPIC_API_KEY;
  if (!apiKey) {
    console.error("ANTHROPIC_API_KEY is required");
    process.exitCode = 1;
    return;
  }

  const threshold = parseInt(process.env.RISK_THRESHOLD ?? "60", 10);
  if (isNaN(threshold)) {
    console.error("RISK_THRESHOLD must be a number");
    process.exitCode = 1;
    return;
  }
  const model = process.env.CLAUDE_MODEL ?? "claude-sonnet-4-20250514";
  const maxOutputTokens = parseInt(process.env.ANTHROPIC_MAX_OUTPUT_TOKENS ?? "4096", 10);
  const maxProposalChars = parseInt(process.env.ANTHROPIC_MAX_PROPOSAL_CHARS ?? "700000", 10);

  const __dirname = dirname(fileURLToPath(import.meta.url));
  const proposalsPath = resolve(__dirname, "eval/proposals.jsonl");
  const goldenSetPath = resolve(__dirname, "eval/golden-set.json");
  const systemPromptPath = resolve(__dirname, "../src/prompts/risk-assessment-system.md");

  const proposals = loadProposals(proposalsPath);
  const goldenSet = loadGoldenSet(goldenSetPath);
  const systemPrompt = readFileSync(systemPromptPath, "utf-8");

  console.log(`Loaded ${proposals.length} proposals, ${Object.keys(goldenSet).length} golden labels`);
  console.log(`Threshold: ${threshold}, Model: ${model}`);
  console.log("");

  const anthropicClient = new Anthropic({ apiKey });
  const aiClient = new ClaudeAIClient(noopLogger, anthropicClient, model, systemPrompt, maxOutputTokens, maxProposalChars);

  const results: EvalResult[] = [];

  for (let i = 0; i < proposals.length; i++) {
    const proposal = proposals[i];
    const label = goldenSet[proposal.sourceId];

    if (!label) {
      console.warn(`Warning: no golden label for sourceId=${proposal.sourceId}, skipping`);
      continue;
    }

    console.log(`[${i + 1}/${proposals.length}] Analyzing: ${proposal.title.substring(0, 60)}...`);

    let assessment: Assessment | undefined;
    try {
      assessment = await aiClient.analyzeProposal({
        proposalTitle: proposal.title,
        proposalText: proposal.rawProposalText,
        proposalUrl: proposal.url,
        proposalType: mapSourceToProposalType(proposal.source),
      });
    } catch (err) {
      console.warn(`  -> SDK threw for ${proposal.sourceId}: ${err}`);
      assessment = undefined;
    }

    if (!assessment) {
      results.push({
        sourceId: proposal.sourceId,
        title: proposal.title,
        humanShouldAlert: label.shouldAlert,
        status: "ai_failure",
      });
    } else {
      // ClaudeAIClient.parseAndValidate() always sets effectiveRisk before returning
      const effectiveRisk = assessment.effectiveRisk!;
      results.push({
        sourceId: proposal.sourceId,
        title: proposal.title,
        humanShouldAlert: label.shouldAlert,
        status: "success",
        riskScore: assessment.riskScore,
        confidence: assessment.confidence,
        effectiveRisk,
        aiAlerts: effectiveRisk >= threshold,
      });
    }
  }

  console.log("");
  printTable(results, threshold, model);
}

void main();

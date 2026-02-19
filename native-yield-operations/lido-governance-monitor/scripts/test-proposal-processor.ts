/**
 * Manual integration test for ProposalProcessor.
 *
 * Fetches NEW/ANALYSIS_FAILED proposals from database → analyzes with AI → persist analysis to DB
 *
 * Prerequisites:
 * 1. Start test database and apply migrations:
 *    make test-db
 *
 * 2. Run the test:
 *    DATABASE_URL=postgresql://testuser:testpass@localhost:5433/lido_governance_monitor_test \
 *    ANTHROPIC_API_KEY=sk-ant-xxx \
 *    pnpm exec tsx scripts/test-proposal-processor.ts
 *
 * Note: Logger is configured for DEBUG level by default. To change, modify the
 * WinstonLogger constructor call in the script.
 *
 * 3. Clean up:
 *    make clean
 */

import Anthropic from "@anthropic-ai/sdk";
import { WinstonLogger } from "@consensys/linea-shared-utils";
import { PrismaPg } from "@prisma/adapter-pg";

import { ClaudeAIClient } from "../src/clients/ClaudeAIClient.js";
import { PrismaClient } from "../prisma/client/client.js";
import { ProposalRepository } from "../src/clients/db/ProposalRepository.js";
import { ProposalSource } from "../src/core/entities/ProposalSource.js";
import { ProposalState } from "../src/core/entities/ProposalState.js";
import { ProposalProcessor } from "../src/services/ProposalProcessor.js";

const TEST_SYSTEM_PROMPT = `You are a security analyst for Linea's Native Yield integration with Lido.

Analyze the governance proposal and respond with a JSON object containing:
- riskScore (0-100): Overall risk score
- riskLevel: "low" | "medium" | "high" | "critical"
- confidence (0-100): Confidence in your assessment
- proposalType: "discourse" | "snapshot" | "onchain_vote"
- impactTypes: Array of "economic" | "technical" | "operational" | "governance-process"
- affectedComponents: Array of "StakingVault" | "VaultHub" | "LazyOracle" | "OperatorGrid" | "PredepositGuarantee" | "Dashboard" | "Other"
- whatChanged: Brief description of changes
- nativeYieldInvariantsAtRisk: Array of "Valid yield reporting" | "User principal protection" | "Pause deposits only on deficit, or liability or ossification" | "Other"
- nativeYieldImpact: Array of concise impact descriptions (one idea per entry)
- recommendedAction: "no-action" | "monitor" | "comment" | "escalate"
- urgency: "none" | "routine" | "urgent" | "critical"
- supportingQuotes: Array of relevant quotes from the proposal
- keyUnknowns: Array of uncertainties

Respond ONLY with valid JSON.`;

const TEST_PROPOSAL = {
  source: ProposalSource.DISCOURSE,
  sourceId: `test-${Date.now()}`,
  url: "https://research.lido.fi/t/test-proposal",
  title: "[Test] Upgrade StakingVault Contract to v2",
  author: "test-author",
  sourceCreatedAt: new Date(),
  rawProposalText: `# Proposal: Upgrade StakingVault Contract to v2

## Summary
This proposal suggests upgrading the StakingVault contract to version 2, which includes:
- New withdrawal mechanism with optimized gas costs
- Updated fee structure (reduced from 10% to 8%)
- Integration with the new LazyOracle for yield calculations

## Motivation
The current StakingVault has been operating for 12 months without issues, but community feedback indicates:
1. Withdrawal times are too long (average 3 days)
2. Gas costs for deposits are higher than competitors
3. Yield reporting has occasional delays

## Technical Changes
- Modify \`withdraw()\` function to use a new batch processing queue
- Update \`calculateYield()\` to query LazyOracle instead of direct calculation
- Add new \`emergencyPause()\` function for security incidents

## Risk Assessment
The upgrade will require a 48-hour timelock and multi-sig approval from 4/7 committee members.

## Timeline
- Snapshot vote: Jan 15-22
- On-chain execution: Jan 25 (if approved)`,
};

async function main() {
  // Check required env vars
  const requiredEnvVars = ["DATABASE_URL", "ANTHROPIC_API_KEY"];
  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    console.error("\nUsage:");
    console.error("  DATABASE_URL=postgresql://testuser:testpass@localhost:5433/lido_governance_monitor_test \\");
    console.error("  ANTHROPIC_API_KEY=sk-ant-xxx \\");
    console.error("  pnpm exec tsx scripts/test-proposal-processor.ts");
    process.exitCode = 1;
    return;
  }

  const databaseUrl = process.env.DATABASE_URL as string;
  const anthropicApiKey = process.env.ANTHROPIC_API_KEY as string;

  const logger = new WinstonLogger("ProposalProcessor.integration", { level: "debug" });
  const adapter = new PrismaPg({ connectionString: databaseUrl });
  const prisma = new PrismaClient({ adapter });

  let testProposalId: string | undefined;

  try {
    // Connect to database
    console.log("\n=== Connecting to database ===");
    await prisma.$connect();
    console.log("Database connected");

    // Create test proposal
    console.log("\n=== Creating test proposal ===");
    const proposalRepository = new ProposalRepository(logger, prisma);
    const created = await proposalRepository.create(TEST_PROPOSAL);
    testProposalId = created.id;
    console.log(`Created proposal: ${created.id}`);
    console.log(`  Title: ${created.title}`);
    console.log(`  State: ${created.state}`);

    // Verify initial state
    const initial = await prisma.proposal.findUnique({ where: { id: testProposalId } });
    if (initial?.state !== ProposalState.NEW) {
      throw new Error(`Expected state NEW, got ${initial?.state}`);
    }
    console.log("Initial state verified: NEW");

    // Create AI client
    console.log("\n=== Initializing AI client ===");
    const anthropicClient = new Anthropic({ apiKey: anthropicApiKey });
    const aiClient = new ClaudeAIClient(
      logger,
      anthropicClient,
      "claude-sonnet-4-20250514",
      TEST_SYSTEM_PROMPT,
      4096,   // maxOutputTokens - same as default in config
      700000,  // maxProposalChars - same as default in config
    );
    console.log("AI client initialized with model: claude-sonnet-4-20250514");

    // Create processor with low threshold
    console.log("\n=== Creating ProposalProcessor ===");
    const riskThreshold = 50; // Low threshold for testing
    const processor = new ProposalProcessor(
      logger,
      aiClient,
      proposalRepository,
      riskThreshold,
      "v1.0.0",
    );
    console.log(`Processor created with risk threshold: ${riskThreshold}`);

    // Run single processing cycle
    console.log("\n=== Running processOnce() ===");
    console.log("Sending proposal to Claude for analysis...");
    const startTime = Date.now();
    await processor.processOnce();
    const elapsed = Date.now() - startTime;
    console.log(`Processing completed in ${elapsed}ms`);

    // Verify results
    console.log("\n=== Verifying results ===");
    const processed = await prisma.proposal.findUnique({ where: { id: testProposalId } });

    if (!processed) {
      throw new Error("Proposal not found after processing");
    }

    console.log(`Final state: ${processed.state}`);
    console.log(`Risk score: ${processed.riskScore}`);
    console.log(`LLM model: ${processed.llmModel}`);
    console.log(`Analysis attempts: ${processed.analysisAttemptCount}`);
    console.log(`Analyzed at: ${processed.analyzedAt?.toISOString()}`);

    // Check state transition
    const validFinalStates = [ProposalState.ANALYZED, ProposalState.ANALYSIS_FAILED];
    if (!validFinalStates.includes(processed.state as ProposalState)) {
      console.error(`\nUnexpected final state: ${processed.state}`);
      console.error(`Expected one of: ${validFinalStates.join(", ")}`);
      process.exitCode = 1;
      return;
    }

    // Check assessment
    if (processed.assessmentJson) {
      console.log("\n=== Assessment JSON ===");
      const assessment = processed.assessmentJson as Record<string, unknown>;
      console.log(`  riskLevel: ${assessment.riskLevel}`);
      console.log(`  confidence: ${assessment.confidence}`);
      console.log(`  recommendedAction: ${assessment.recommendedAction}`);
      console.log(`  urgency: ${assessment.urgency}`);
      console.log(`  impactTypes: ${JSON.stringify(assessment.impactTypes)}`);
      console.log(`  affectedComponents: ${JSON.stringify(assessment.affectedComponents)}`);
      console.log(`  whatChanged: ${String(assessment.whatChanged).substring(0, 100)}...`);
    } else {
      console.error("\nNo assessment JSON found - analysis may have failed");
      process.exitCode = 1;
      return;
    }

    // Summary
    console.log("\n=== Test Summary ===");
    console.log(`Proposal ID: ${testProposalId}`);
    console.log(`State transition: NEW -> ${processed.state}`);
    console.log(`Risk score: ${processed.riskScore} (threshold: ${riskThreshold})`);
    console.log(
      `Notification would be sent: ${processed.riskScore && processed.riskScore >= riskThreshold ? "YES" : "NO"}`,
    );
    console.log("\n=== All tests passed ===");
  } catch (err) {
    console.error("\nProposalProcessor integration test failed:", err);
    process.exitCode = 1;
  } finally {
    // Clean up test data
    if (testProposalId) {
      console.log("\n=== Cleaning up test data ===");
      try {
        await prisma.proposal.delete({ where: { id: testProposalId } });
        console.log(`Deleted test proposal: ${testProposalId}`);
      } catch (cleanupErr) {
        console.warn("Failed to clean up test proposal:", cleanupErr);
      }
    }

    // Disconnect
    await prisma.$disconnect();
    console.log("Database disconnected");
  }
}

void main();

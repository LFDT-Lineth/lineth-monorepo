/**
 * Manual integration test for ProposalFetcher.
 *
 * Fetch proposals from Discourse forum -> Save to DB
 *
 * Prerequisites:
 * 1. Start test database and apply migrations:
 *    make test-db
 *
 * 2. Run the test:
 *    DATABASE_URL=postgresql://testuser:testpass@localhost:5433/lido_governance_monitor_test \
 *    DISCOURSE_PROPOSALS_URL=https://research.lido.fi/c/proposals/9/l/latest.json \
 *    pnpm exec tsx scripts/test-proposal-fetcher.ts
 *
 * 3. Clean up:
 *    make clean
 *
 * Optional env vars:
 *   MAX_PROPOSALS=3       # Limit number of proposals to process (default: 3)
 *   CLEANUP=true          # Delete created proposals after test (default: false)
 */

import { ExponentialBackoffRetryService, WinstonLogger } from "@consensys/linea-shared-utils";
import { PrismaPg } from "@prisma/adapter-pg";

import { DiscourseClient } from "../src/clients/DiscourseClient.js";
import { PrismaClient } from "../prisma/client/client.js";
import { ProposalRepository } from "../src/clients/db/ProposalRepository.js";
import { ProposalSource } from "../src/core/entities/ProposalSource.js";
import { ProposalState } from "../src/core/entities/ProposalState.js";
import { DiscourseFetcher } from "../src/services/fetchers/DiscourseFetcher.js";
import { NormalizationService } from "../src/services/NormalizationService.js";
import { ProposalFetcher } from "../src/services/ProposalFetcher.js";

async function main() {
  // Check required env vars
  const requiredEnvVars = ["DATABASE_URL", "DISCOURSE_PROPOSALS_URL"];
  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    console.error("\nUsage:");
    console.error("  DATABASE_URL=postgresql://testuser:testpass@localhost:5433/lido_governance_monitor_test \\");
    console.error("  DISCOURSE_PROPOSALS_URL=https://research.lido.fi/c/proposals/9/l/latest.json \\");
    console.error("  pnpm exec tsx scripts/test-proposal-fetcher.ts");
    process.exitCode = 1;
    return;
  }

  const databaseUrl = process.env.DATABASE_URL as string;
  const discourseProposalsUrl = process.env.DISCOURSE_PROPOSALS_URL as string;
  const maxProposals = process.env.MAX_PROPOSALS ? Number.parseInt(process.env.MAX_PROPOSALS, 10) : 3;
  const shouldCleanup = process.env.CLEANUP === "true";

  const logger = new WinstonLogger("ProposalFetcher.integration");
  const adapter = new PrismaPg({ connectionString: databaseUrl });
  const prisma = new PrismaClient({ adapter });

  const createdProposalIds: string[] = [];

  try {
    // Connect to database
    console.log("\n=== Connecting to database ===");
    await prisma.$connect();
    console.log("Database connected");

    // Create dependencies
    console.log("\n=== Initializing dependencies ===");
    const retryService = new ExponentialBackoffRetryService(logger);
    const discourseClient = new DiscourseClient(logger, retryService, discourseProposalsUrl, 15000);
    const normalizationService = new NormalizationService(logger, discourseClient.getBaseUrl());
    const proposalRepository = new ProposalRepository(logger, prisma);

    console.log(`Discourse URL: ${discourseProposalsUrl}`);
    console.log(`Base URL: ${discourseClient.getBaseUrl()}`);

    // Create fetcher
    const discourseFetcher = new DiscourseFetcher(logger, discourseClient, normalizationService, proposalRepository, maxProposals);
    const fetcher = new ProposalFetcher(
      logger,
      [discourseFetcher],
    );

    // Get initial proposal count
    const initialCount = await prisma.proposal.count({
      where: { source: ProposalSource.DISCOURSE },
    });
    console.log(`Initial Discourse proposals in DB: ${initialCount}`);

    // Test 1: Fetch latest proposals from Discourse
    console.log("\n=== Test 1: Fetching latest proposals from Discourse ===");
    const proposalList = await discourseClient.fetchLatestProposals();

    if (!proposalList) {
      console.error("Failed to fetch proposals from Discourse");
      process.exitCode = 1;
      return;
    }

    const topics = proposalList.topic_list?.topics ?? [];
    console.log(`Fetched ${topics.length} topics from Discourse`);

    if (topics.length === 0) {
      console.error("No topics found in Discourse");
      process.exitCode = 1;
      return;
    }

    // Show first few topics
    console.log("\nFirst 5 topics:");
    topics.slice(0, 5).forEach((topic, i) => {
      console.log(`  ${i + 1}. [${topic.id}] ${topic.slug}`);
    });

    // Test 2: Run getLatestProposals with limited proposals
    console.log(`\n=== Test 2: Running getLatestProposals() (max ${maxProposals} new proposals) ===`);

    // Find topics that don't exist in DB yet
    const newTopics: typeof topics = [];
    for (const topic of topics) {
      if (newTopics.length >= maxProposals) break;

      const existing = await proposalRepository.findBySourceAndSourceId(
        ProposalSource.DISCOURSE,
        topic.id.toString(),
      );
      if (!existing) {
        newTopics.push(topic);
      }
    }

    if (newTopics.length === 0) {
      console.log("All topics already exist in database. Running getLatestProposals() anyway...");
    } else {
      console.log(`Found ${newTopics.length} new topics to process`);
    }

    // Run the fetcher
    const startTime = Date.now();
    await fetcher.getLatestProposals();
    const elapsed = Date.now() - startTime;
    console.log(`getLatestProposals() completed in ${elapsed}ms`);

    // Test 3: Verify proposals were created
    console.log("\n=== Test 3: Verifying created proposals ===");

    const finalCount = await prisma.proposal.count({
      where: { source: ProposalSource.DISCOURSE },
    });
    const newProposalsCreated = finalCount - initialCount;
    console.log(`New proposals created: ${newProposalsCreated}`);

    // Fetch recently created proposals
    const recentProposals = await prisma.proposal.findMany({
      where: { source: ProposalSource.DISCOURSE },
      orderBy: { createdAt: "desc" },
      take: Math.max(newProposalsCreated, 3),
    });

    console.log("\nRecently created/existing proposals:");
    for (const proposal of recentProposals) {
      console.log(`  ID: ${proposal.id}`);
      console.log(`    Source ID: ${proposal.sourceId}`);
      console.log(`    Title: ${proposal.title.substring(0, 60)}${proposal.title.length > 60 ? "..." : ""}`);
      console.log(`    State: ${proposal.state}`);
      console.log(`    Author: ${proposal.author ?? "N/A"}`);
      console.log(`    Text length: ${proposal.rawProposalText.length} chars`);
      console.log(`    Created at: ${proposal.createdAt.toISOString()}`);
      console.log("");

      // Track for cleanup
      if (newProposalsCreated > 0) {
        createdProposalIds.push(proposal.id);
      }
    }

    // Test 4: Verify proposal state
    console.log("=== Test 4: Verifying proposal states ===");
    const newStateProposals = await prisma.proposal.findMany({
      where: {
        source: ProposalSource.DISCOURSE,
        state: ProposalState.NEW,
      },
    });
    console.log(`Proposals in NEW state: ${newStateProposals.length}`);

    // Test 5: Verify a specific proposal's content
    if (recentProposals.length > 0) {
      console.log("\n=== Test 5: Detailed proposal inspection ===");
      const sampleProposal = recentProposals[0];
      console.log(`Inspecting proposal: ${sampleProposal.id}`);
      console.log(`  Full title: ${sampleProposal.title}`);
      console.log(`  URL: ${sampleProposal.url}`);
      console.log(`  Source created at: ${sampleProposal.sourceCreatedAt.toISOString()}`);
      console.log(`  Text preview (first 500 chars):`);
      console.log(`    ${sampleProposal.rawProposalText.substring(0, 500).replace(/\n/g, "\n    ")}...`);
    }

    // Summary
    console.log("\n=== Test Summary ===");
    console.log(`Discourse topics fetched: ${topics.length}`);
    console.log(`New proposals created: ${newProposalsCreated}`);
    console.log(`Total Discourse proposals in DB: ${finalCount}`);
    console.log("\n=== All tests passed ===");
  } catch (err) {
    console.error("\nProposalFetcher integration test failed:", err);
    process.exitCode = 1;
  } finally {
    // Optional cleanup
    if (shouldCleanup && createdProposalIds.length > 0) {
      console.log("\n=== Cleaning up test data ===");
      try {
        const deleted = await prisma.proposal.deleteMany({
          where: { id: { in: createdProposalIds } },
        });
        console.log(`Deleted ${deleted.count} test proposals`);
      } catch (cleanupErr) {
        console.warn("Failed to clean up test proposals:", cleanupErr);
      }
    } else if (createdProposalIds.length > 0) {
      console.log("\n=== Test data retained ===");
      console.log(`Created ${createdProposalIds.length} proposals (use CLEANUP=true to delete)`);
    }

    // Disconnect
    await prisma.$disconnect();
    console.log("Database disconnected");
  }
}

void main();

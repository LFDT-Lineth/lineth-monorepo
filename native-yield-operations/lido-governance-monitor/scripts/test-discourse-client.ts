/**
 * Manual integration test for DiscourseClient.
 *
 * Fetches latest proposals from Discourse API 
 *
 * Example usage:
 * DISCOURSE_PROPOSALS_URL=https://research.lido.fi/c/proposals/9/l/latest.json \
 * pnpm --filter @consensys/lido-governance-monitor exec tsx scripts/test-discourse-client.ts
 *
 * Optional env vars:
 * PROPOSAL_ID=11107    # Specific proposal ID to fetch details for
 */

import { ExponentialBackoffRetryService, WinstonLogger } from "@consensys/linea-shared-utils";
import { DiscourseClient } from "../src/clients/DiscourseClient.js";

async function main() {
  const requiredEnvVars = ["DISCOURSE_PROPOSALS_URL"];

  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }

  const proposalsUrl = process.env.DISCOURSE_PROPOSALS_URL as string;
  const proposalId = process.env.PROPOSAL_ID ? Number.parseInt(process.env.PROPOSAL_ID, 10) : undefined;

  const logger = new WinstonLogger("DiscourseClient.integration");
  const retryService = new ExponentialBackoffRetryService(logger);
  const client = new DiscourseClient(logger, retryService, proposalsUrl, 15000);

  try {
    // Test 1: Fetch latest proposals
    console.log("\n=== Fetching latest proposals ===");
    const proposals = await client.fetchLatestProposals();

    if (!proposals) {
      console.error("Failed to fetch latest proposals");
      process.exitCode = 1;
      return;
    }

    const topics = proposals.topic_list?.topics ?? [];
    console.log(`Found ${topics.length} proposals`);
    console.log("First 5 proposals:");
    topics.slice(0, 5).forEach((topic, i) => {
      console.log(`  ${i + 1}. [${topic.id}] ${topic.slug}`);
    });

    // Test 2: Fetch proposal details
    const targetId = proposalId ?? topics[0]?.id;
    if (targetId) {
      console.log(`\n=== Fetching proposal details for ID: ${targetId} ===`);
      const details = await client.fetchProposalDetails(targetId);

      if (!details) {
        console.error(`Failed to fetch proposal details for ID: ${targetId}`);
        process.exitCode = 1;
        return;
      }

      console.log(`Title: ${details.title}`);
      console.log(`Created: ${details.created_at}`);
      console.log(`Posts: ${details.post_stream?.posts?.length ?? 0}`);

      const firstPost = details.post_stream?.posts?.[0];
      if (firstPost) {
        console.log(`Author: ${firstPost.username}`);
        console.log(`Content preview: ${firstPost.cooked.substring(0, 200)}...`);
      }
    }

    console.log("\n=== All tests passed ===");
  } catch (err) {
    console.error("DiscourseClient integration test failed:", err);
    process.exitCode = 1;
  }
}

void main();

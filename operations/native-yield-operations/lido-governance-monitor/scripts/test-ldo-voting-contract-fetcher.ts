/**
 * Manual integration test for LdoVotingContractFetcher.
 *
 * Fetches vote proposals from LDO AragonVoting contract
 *
 * Example usage:
 * ETHEREUM_RPC_URL=https://mainnet.infura.io/v3/YOUR_KEY \
 * pnpm --filter @consensys/lido-governance-monitor exec tsx scripts/test-ldo-voting-contract-fetcher.ts
 *
 * Optional env vars:
 * LDO_VOTING_CONTRACT_ADDRESS=0x2e59a20f205bb85a89c53f1936454680651e618e  # Default
 * INITIAL_LDO_VOTING_CONTRACT_VOTEID=150  # Vote ID to start scanning from (default: 0)
 */

import { createPublicClient, http, type Address } from "viem";
import { mainnet } from "viem/chains";

import { IProposalRepository } from "../src/core/repositories/IProposalRepository.js";
import { LdoVotingContractFetcher } from "../src/services/fetchers/LdoVotingContractFetcher.js";
import { createLidoGovernanceMonitorLogger } from "../src/utils/logging/index.js";

async function main() {
  const requiredEnvVars = ["ETHEREUM_RPC_URL"];

  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }

  const rpcUrl = process.env.ETHEREUM_RPC_URL as string;
  const contractAddress = (process.env.LDO_VOTING_CONTRACT_ADDRESS ??
    "0x2e59a20f205bb85a89c53f1936454680651e618e") as Address;
  const initialVoteId = process.env.INITIAL_LDO_VOTING_CONTRACT_VOTEID
    ? BigInt(process.env.INITIAL_LDO_VOTING_CONTRACT_VOTEID)
    : undefined;

  const logger = createLidoGovernanceMonitorLogger("LdoVotingContractFetcher.integration");

  console.log("\n=== LdoVotingContractFetcher Integration Test ===");
  console.log(`RPC URL: ${rpcUrl.replace(/\/[^/]+$/, "/***")}`);
  console.log(`Contract: ${contractAddress}`);
  console.log(`Initial vote ID: ${initialVoteId ?? "0 (default)"}`);

  try {
    // Step 1: Create viem client
    console.log("\n--- Creating Ethereum client ---");
    const publicClient = createPublicClient({
      chain: mainnet,
      transport: http(rpcUrl),
    });

    const blockNumber = await publicClient.getBlockNumber();
    console.log(`Connected. Current block: ${blockNumber}`);

    // Step 2: Fetch proposals
    console.log("\n--- Fetching votes via readContract + narrow getLogs ---");
    // Stub repository — no DB needed for this manual RPC test
    const stubRepository = { findLatestSourceIdBySource: async () => null } as unknown as IProposalRepository;
    const fetcher = new LdoVotingContractFetcher(logger, publicClient, contractAddress, initialVoteId, stubRepository);

    const proposals = await fetcher.getLatestProposals();
    console.log(`Fetched ${proposals.length} proposals`);

    if (proposals.length === 0) {
      console.warn("No proposals returned. Check contract address and RPC connection.");
      process.exitCode = 1;
      return;
    }

    // Step 3: Print results
    console.log("\n--- Proposals ---");
    for (const proposal of proposals) {
      console.log(`\n  Vote #${proposal.sourceId}`);
      console.log(`    Title:     ${proposal.title}`);
      console.log(`    Source:    ${proposal.source}`);
      console.log(`    Author:   ${proposal.author}`);
      console.log(`    Created:  ${proposal.sourceCreatedAt.toISOString()}`);
      console.log(`    URL:      ${proposal.url}`);
      console.log(`    Metadata: ${proposal.rawProposalText.substring(0, 200)}${proposal.rawProposalText.length > 200 ? "..." : ""}`);
    }

    // Step 4: Validate fields
    console.log("\n--- Validating fields ---");
    let valid = true;
    for (const proposal of proposals) {
      const issues: string[] = [];
      if (!proposal.sourceId) issues.push("missing sourceId");
      if (!proposal.title) issues.push("missing title");
      if (!proposal.url) issues.push("missing url");
      if (!proposal.source) issues.push("missing source");
      if (!proposal.sourceCreatedAt) issues.push("missing sourceCreatedAt");
      if (proposal.rawProposalText === undefined || proposal.rawProposalText === null) issues.push("missing text");

      if (issues.length > 0) {
        console.error(`  Vote #${proposal.sourceId}: ${issues.join(", ")}`);
        valid = false;
      }
    }

    if (valid) {
      console.log("  All fields valid.");
    } else {
      process.exitCode = 1;
      return;
    }

    console.log("\n=== All tests passed ===");
  } catch (err) {
    console.error("LdoVotingContractFetcher integration test failed:", err);
    process.exitCode = 1;
  }
}

void main();

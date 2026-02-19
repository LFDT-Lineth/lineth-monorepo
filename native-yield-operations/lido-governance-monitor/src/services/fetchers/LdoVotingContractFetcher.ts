import { type Address, type PublicClient, parseAbiItem } from "viem";

import { CreateProposalInput } from "../../core/entities/Proposal.js";
import { ProposalSource } from "../../core/entities/ProposalSource.js";
import { IProposalRepository } from "../../core/repositories/IProposalRepository.js";
import { IProposalFetcher } from "../../core/services/IProposalFetcher.js";
import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";

const START_VOTE_EVENT = parseAbiItem(
  "event StartVote(uint256 indexed voteId, address indexed creator, string metadata)",
);

const VOTES_LENGTH_ABI = parseAbiItem("function votesLength() view returns (uint64)");

const GET_VOTE_ABI = parseAbiItem(
  "function getVote(uint256 _voteId) view returns (bool open, bool executed, uint64 startDate, uint64 snapshotBlock, uint64 supportRequired, uint64 minAcceptQuorum, uint256 yea, uint256 nay, uint256 votingPower, bytes script, uint8 phase)",
);

export class LdoVotingContractFetcher implements IProposalFetcher {
  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly publicClient: PublicClient,
    private readonly contractAddress: Address,
    private readonly initialVoteId: bigint | undefined,
    private readonly proposalRepository: IProposalRepository,
  ) {}

  async getLatestProposals(): Promise<CreateProposalInput[]> {
    let votesLength: bigint;
    try {
      votesLength = await this.publicClient.readContract({
        address: this.contractAddress,
        abi: [VOTES_LENGTH_ABI],
        functionName: "votesLength",
      });
    } catch (error) {
      this.logger.critical("Failed to call votesLength on LDO voting contract", {
        error: error instanceof Error ? error.message : String(error),
      });
      return [];
    }

    if (votesLength === 0n) {
      return [];
    }

    const latestDbSourceId = await this.proposalRepository.findLatestSourceIdBySource(
      ProposalSource.LDO_VOTING_CONTRACT,
    );

    const startVoteId = latestDbSourceId != null ? Number(latestDbSourceId) + 1 : Number(this.initialVoteId ?? 0n);
    const endVoteId = Number(votesLength) - 1;

    const proposals: CreateProposalInput[] = [];

    for (let voteId = startVoteId; voteId <= endVoteId; voteId++) {
      try {
        const voteData = await this.publicClient.readContract({
          address: this.contractAddress,
          abi: [GET_VOTE_ABI],
          functionName: "getVote",
          args: [BigInt(voteId)],
        });

        const startDate = (voteData as readonly unknown[])[2] as bigint;
        const snapshotBlock = (voteData as readonly unknown[])[3] as bigint;

        // CRITICAL ASSUMPTION: We query a 10-block window (snapshotBlock to snapshotBlock + 9)
        // for the StartVote event. On-chain observation shows the event fires exactly 1 block
        // after the vote proposal tx, so 10 blocks provides a safety margin. The window is kept
        // small because some RPC providers reject getLogs calls spanning larger block ranges.
        // If the event lands outside this window, `creator` and `metadata` will be null/empty.
        const logs = await this.publicClient.getLogs({
          address: this.contractAddress,
          event: START_VOTE_EVENT,
          args: { voteId: BigInt(voteId) },
          fromBlock: snapshotBlock,
          toBlock: snapshotBlock + 9n,
        });

        const creator = logs.length > 0 ? (logs[0].args.creator ?? null) : null;
        const metadata = logs.length > 0 ? (logs[0].args.metadata ?? "") : "";

        const proposal = {
          source: ProposalSource.LDO_VOTING_CONTRACT,
          sourceId: String(voteId),
          url: `https://vote.lido.fi/vote/${voteId}`,
          title: `LDO Contract vote ${voteId}`,
          author: creator,
          sourceCreatedAt: new Date(Number(startDate) * 1000),
          rawProposalText: metadata,
        };

        // Gap-prevention: votes must be persisted in strict sequential order.
        // On the next run, findLatestSourceIdBySource returns the highest persisted
        // vote ID, and fetching resumes from ID+1. If vote N fails to persist but
        // N+1 succeeds, vote N is permanently skipped.
        //
        // We persist here (not in ProposalFetcher) because ProposalFetcher's
        // persistence loop catches DB errors and continues to the next proposal.
        // By persisting inside this try block, a DB failure triggers the break
        // below - halting the loop so no later vote can leapfrog a failed one.
        const { isNew } = await this.proposalRepository.upsert(proposal);
        if (isNew) {
          this.logger.info("Created new LDO vote", { sourceId: String(voteId) });
        } else {
          this.logger.debug("LDO vote already exists, skipping", { sourceId: String(voteId) });
        }
        proposals.push(proposal);
      } catch (error) {
        this.logger.critical(`Failed to fetch/persist vote ${voteId}, stopping to prevent gap`, {
          error: error instanceof Error ? error.message : String(error),
        });
        break;
      }
    }

    return proposals;
  }
}

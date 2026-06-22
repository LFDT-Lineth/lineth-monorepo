import { CreateProposalInput } from "../core/entities/Proposal.js";
import { IProposalFetcher } from "../core/services/IProposalFetcher.js";
import { ILidoGovernanceMonitorLogger } from "../utils/logging/index.js";

export class ProposalFetcher implements IProposalFetcher {
  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly sourceFetchers: IProposalFetcher[],
  ) {}

  async getLatestProposals(): Promise<CreateProposalInput[]> {
    this.logger.info("Starting proposal polling");

    const results = await Promise.allSettled(this.sourceFetchers.map((fetcher) => fetcher.getLatestProposals()));

    const proposals: CreateProposalInput[] = [];
    for (const result of results) {
      if (result.status === "fulfilled") {
        proposals.push(...result.value);
      } else {
        this.logger.critical("Source fetcher failed", { error: result.reason });
      }
    }

    this.logger.info("Proposal polling completed", {
      sources: this.sourceFetchers.length,
      fetched: proposals.length,
    });

    return proposals;
  }
}

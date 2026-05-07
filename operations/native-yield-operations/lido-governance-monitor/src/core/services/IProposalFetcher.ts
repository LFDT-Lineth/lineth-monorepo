import { CreateProposalInput } from "../entities/Proposal.js";

export interface IProposalFetcher {
  getLatestProposals(): Promise<CreateProposalInput[]>;
}

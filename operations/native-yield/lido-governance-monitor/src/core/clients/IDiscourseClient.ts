import { RawDiscourseProposal, RawDiscourseProposalList } from "../entities/RawDiscourseProposal.js";

export interface IDiscourseClient {
  fetchLatestProposals(): Promise<RawDiscourseProposalList | undefined>;
  fetchProposalDetails(proposalId: number): Promise<RawDiscourseProposal | undefined>;
  getBaseUrl(): string;
}

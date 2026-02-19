import { CreateProposalInput } from "../entities/Proposal.js";
import { RawDiscourseProposal } from "../entities/RawDiscourseProposal.js";

export interface INormalizationService {
  normalizeDiscourseProposal(proposal: RawDiscourseProposal): CreateProposalInput;
  stripHtml(html: string): string;
}

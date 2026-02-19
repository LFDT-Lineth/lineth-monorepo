import { Assessment } from "../entities/Assessment.js";
import { ProposalWithoutText } from "../entities/Proposal.js";

export interface SlackNotificationResult {
  success: boolean;
  error?: string;
}

export interface ISlackClient {
  sendProposalAlert(proposal: ProposalWithoutText, assessment: Assessment): Promise<SlackNotificationResult>;
  sendAuditLog(proposal: ProposalWithoutText, assessment: Assessment): Promise<SlackNotificationResult>;
}

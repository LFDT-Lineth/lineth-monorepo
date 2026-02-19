import { Assessment } from "./Assessment.js";
import { ProposalSource } from "./ProposalSource.js";
import { ProposalState } from "./ProposalState.js";

export interface Proposal {
  id: string;
  source: ProposalSource;
  sourceId: string;
  createdAt: Date;
  updatedAt: Date;
  url: string;
  title: string;
  author: string | null;
  sourceCreatedAt: Date;
  rawProposalText: string;
  sourceMetadata: unknown | null;
  state: ProposalState;
  stateUpdatedAt: Date;
  analysisAttemptCount: number;
  llmModel: string | null;
  riskThreshold: number | null;
  assessmentPromptVersion: string | null;
  analyzedAt: Date | null;
  assessmentJson: Assessment | null;
  riskScore: number | null;
  notifyAttemptCount: number;
  notifiedAt: Date | null;
}

export type ProposalWithoutText = Omit<Proposal, "rawProposalText">;

export interface CreateProposalInput {
  source: ProposalSource;
  sourceId: string;
  url: string;
  title: string;
  author: string | null;
  sourceCreatedAt: Date;
  rawProposalText: string;
}

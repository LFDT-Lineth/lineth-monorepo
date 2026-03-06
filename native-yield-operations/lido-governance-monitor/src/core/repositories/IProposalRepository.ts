import { Assessment } from "../entities/Assessment.js";
import { Proposal, ProposalWithoutText, CreateProposalInput } from "../entities/Proposal.js";
import { ProposalSource } from "../entities/ProposalSource.js";
import { ProposalState } from "../entities/ProposalState.js";

export interface IProposalRepository {
  findBySourceAndSourceId(source: ProposalSource, sourceId: string): Promise<Proposal | null>;
  findByStateForAnalysis(state: ProposalState, maxAnalysisAttempts?: number): Promise<Proposal[]>;
  findByStateForNotification(state: ProposalState, maxNotifyAttempts?: number): Promise<ProposalWithoutText[]>;
  create(input: CreateProposalInput): Promise<Proposal>;
  upsert(input: CreateProposalInput): Promise<{ proposal: Proposal; isNew: boolean }>;
  updateState(id: string, state: ProposalState): Promise<Proposal>;
  attemptUpdateState(id: string, state: ProposalState): Promise<Proposal | null>;
  saveAnalysis(
    id: string,
    assessment: Assessment,
    riskScore: number,
    llmModel: string,
    riskThreshold: number,
    promptVersion: string,
  ): Promise<Proposal>;
  incrementAnalysisAttempt(id: string): Promise<Proposal>;
  markNotified(id: string): Promise<Proposal>;
  markNotifyFailed(id: string): Promise<Proposal>;
  attemptMarkNotifyFailed(id: string): Promise<Proposal | null>;
  findLatestSourceIdBySource(source: ProposalSource): Promise<string | null>;
}

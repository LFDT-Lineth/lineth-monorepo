import { PrismaClient } from "../../../prisma/client/client.js";
import { Assessment } from "../../core/entities/Assessment.js";
import { Proposal, ProposalWithoutText, CreateProposalInput } from "../../core/entities/Proposal.js";
import { ProposalSource } from "../../core/entities/ProposalSource.js";
import { ProposalState } from "../../core/entities/ProposalState.js";
import { IProposalRepository } from "../../core/repositories/IProposalRepository.js";
import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";

export class ProposalRepository implements IProposalRepository {
  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly prisma: PrismaClient,
  ) {}

  async findBySourceAndSourceId(source: ProposalSource, sourceId: string): Promise<Proposal | null> {
    return this.prisma.proposal.findUnique({
      where: { source_sourceId: { source, sourceId } },
    }) as Promise<Proposal | null>;
  }

  async findByStateForAnalysis(state: ProposalState, maxAnalysisAttempts?: number): Promise<Proposal[]> {
    return this.prisma.proposal.findMany({
      where: {
        state,
        ...(maxAnalysisAttempts !== undefined && { analysisAttemptCount: { lt: maxAnalysisAttempts } }),
      },
      orderBy: { stateUpdatedAt: "asc" },
    }) as Promise<Proposal[]>;
  }

  async findByStateForNotification(state: ProposalState, maxNotifyAttempts?: number): Promise<ProposalWithoutText[]> {
    return this.prisma.proposal.findMany({
      where: {
        state,
        ...(maxNotifyAttempts !== undefined && { notifyAttemptCount: { lt: maxNotifyAttempts } }),
      },
      orderBy: { stateUpdatedAt: "asc" },
      omit: { rawProposalText: true },
    }) as Promise<ProposalWithoutText[]>;
  }

  async create(input: CreateProposalInput): Promise<Proposal> {
    return this.prisma.proposal.create({
      data: {
        source: input.source,
        sourceId: input.sourceId,
        url: input.url,
        title: input.title,
        author: input.author,
        sourceCreatedAt: input.sourceCreatedAt,
        rawProposalText: input.rawProposalText,
        state: ProposalState.NEW,
        stateUpdatedAt: new Date(),
      },
    }) as Promise<Proposal>;
  }

  async upsert(input: CreateProposalInput): Promise<{ proposal: Proposal; isNew: boolean }> {
    const existing = await this.findBySourceAndSourceId(input.source, input.sourceId);
    if (existing) {
      return { proposal: existing, isNew: false };
    }
    const proposal = await this.create(input);
    return { proposal, isNew: true };
  }

  async updateState(id: string, state: ProposalState): Promise<Proposal> {
    return this.prisma.proposal.update({
      where: { id },
      data: { state, stateUpdatedAt: new Date() },
    }) as Promise<Proposal>;
  }

  async attemptUpdateState(id: string, state: ProposalState): Promise<Proposal | null> {
    try {
      return await this.updateState(id, state);
    } catch (error) {
      this.logger.critical("attemptUpdateState failed", { id, state, error });
      return null;
    }
  }

  async saveAnalysis(
    id: string,
    assessment: Assessment,
    riskScore: number,
    llmModel: string,
    riskThreshold: number,
    promptVersion: string,
  ): Promise<Proposal> {
    return this.prisma.proposal.update({
      where: { id },
      data: {
        state: ProposalState.ANALYZED,
        stateUpdatedAt: new Date(),
        assessmentJson: assessment as object,
        riskScore,
        llmModel,
        riskThreshold,
        assessmentPromptVersion: promptVersion,
        analyzedAt: new Date(),
      },
    }) as Promise<Proposal>;
  }

  async incrementAnalysisAttempt(id: string): Promise<Proposal> {
    return this.prisma.proposal.update({
      where: { id },
      data: { analysisAttemptCount: { increment: 1 } },
    }) as Promise<Proposal>;
  }

  // Orders by sourceCreatedAt (not sourceId) because sourceId is a string type
  // and may not sort numerically. If two proposals share the same timestamp, the
  // returned sourceId may not be the numerically highest, but this is acceptable -
  // callers like LdoVotingContractFetcher handle redundant re-fetching gracefully.
  async findLatestSourceIdBySource(source: ProposalSource): Promise<string | null> {
    const result = await this.prisma.proposal.findFirst({
      where: { source },
      orderBy: { sourceCreatedAt: "desc" },
      select: { sourceId: true },
    });
    return result?.sourceId ?? null;
  }

  async markNotified(id: string): Promise<Proposal> {
    return this.prisma.proposal.update({
      where: { id },
      data: {
        state: ProposalState.NOTIFIED,
        stateUpdatedAt: new Date(),
        notifiedAt: new Date(),
        notifyAttemptCount: { increment: 1 },
      },
    }) as Promise<Proposal>;
  }

  async markNotifyFailed(id: string): Promise<Proposal> {
    return this.prisma.proposal.update({
      where: { id },
      data: {
        state: ProposalState.NOTIFY_FAILED,
        stateUpdatedAt: new Date(),
        notifyAttemptCount: { increment: 1 },
      },
    }) as Promise<Proposal>;
  }

  async attemptMarkNotifyFailed(id: string): Promise<Proposal | null> {
    try {
      return await this.markNotifyFailed(id);
    } catch (error) {
      this.logger.critical("attemptMarkNotifyFailed failed", { id, error });
      return null;
    }
  }
}

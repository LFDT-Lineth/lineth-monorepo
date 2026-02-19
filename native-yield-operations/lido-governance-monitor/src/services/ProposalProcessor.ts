import { IAIClient } from "../core/clients/IAIClient.js";
import { ProposalType } from "../core/entities/Assessment.js";
import { Proposal } from "../core/entities/Proposal.js";
import { ProposalSource } from "../core/entities/ProposalSource.js";
import { ProposalState } from "../core/entities/ProposalState.js";
import { IProposalRepository } from "../core/repositories/IProposalRepository.js";
import { IProposalProcessor } from "../core/services/IProposalProcessor.js";
import { ILidoGovernanceMonitorLogger } from "../utils/logging/index.js";

export class ProposalProcessor implements IProposalProcessor {
  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly aiClient: IAIClient,
    private readonly proposalRepository: IProposalRepository,
    private readonly riskThreshold: number,
    private readonly promptVersion: string,
    private readonly maxAnalysisAttempts: number,
  ) {}

  async processOnce(): Promise<void> {
    try {
      this.logger.info("Starting proposal processing");

      const newProposals = await this.proposalRepository.findByStateForAnalysis(
        ProposalState.NEW,
        this.maxAnalysisAttempts,
      );
      const failedProposals = await this.proposalRepository.findByStateForAnalysis(
        ProposalState.ANALYSIS_FAILED,
        this.maxAnalysisAttempts,
      );
      const proposals = [...newProposals, ...failedProposals];

      if (proposals.length === 0) {
        this.logger.debug("No proposals to process");
        return;
      }

      this.logger.debug("Processing proposals", { count: proposals.length });

      for (const proposal of proposals) {
        await this.processProposal(proposal);
      }

      this.logger.info("Proposal processing completed");
    } catch (error) {
      this.logger.critical("Proposal processing failed", { error });
    }
  }

  private async processProposal(proposal: Proposal): Promise<void> {
    try {
      // Increment attempt count before analysis
      const updated = await this.proposalRepository.incrementAnalysisAttempt(proposal.id);

      // Perform AI analysis
      const assessment = await this.aiClient.analyzeProposal({
        proposalTitle: proposal.title,
        proposalText: proposal.rawProposalText,
        proposalUrl: proposal.url,
        proposalType: this.mapSourceToProposalType(proposal.source),
      });

      if (!assessment) {
        // Analysis failed - transition to ANALYSIS_FAILED for retry
        await this.proposalRepository.updateState(proposal.id, ProposalState.ANALYSIS_FAILED);
        this.logger.warn("AI analysis failed, will retry", {
          proposalId: proposal.id,
          attempt: updated.analysisAttemptCount,
        });
        return;
      }

      // Save analysis result
      await this.proposalRepository.saveAnalysis(
        proposal.id,
        assessment,
        assessment.riskScore,
        this.aiClient.getModelName(),
        this.riskThreshold,
        this.promptVersion,
      );

      this.logger.info("Proposal analysis completed", {
        proposalId: proposal.id,
        riskScore: assessment.riskScore,
      });
      this.logger.debug("Full assessment details", {
        proposalId: proposal.id,
        assessment,
      });
    } catch (error) {
      // Best-effort transition so the proposal doesn't silently get stuck in NEW
      // when analysisAttemptCount was already incremented above but state was never updated.
      await this.proposalRepository.attemptUpdateState(proposal.id, ProposalState.ANALYSIS_FAILED);
      this.logger.critical("Error processing proposal", { proposalId: proposal.id, error });
    }
  }

  private mapSourceToProposalType(source: ProposalSource): ProposalType {
    switch (source) {
      case ProposalSource.DISCOURSE:
        return "discourse";
      case ProposalSource.SNAPSHOT:
        return "snapshot";
      case ProposalSource.LDO_VOTING_CONTRACT:
      case ProposalSource.STETH_VOTING_CONTRACT:
        return "onchain_vote";
    }
  }
}

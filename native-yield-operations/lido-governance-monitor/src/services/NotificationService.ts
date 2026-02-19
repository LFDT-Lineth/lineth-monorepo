import { ISlackClient } from "../core/clients/ISlackClient.js";
import { AssessmentSchema } from "../core/entities/Assessment.js";
import { ProposalWithoutText } from "../core/entities/Proposal.js";
import { ProposalState } from "../core/entities/ProposalState.js";
import { IProposalRepository } from "../core/repositories/IProposalRepository.js";
import { INotificationService } from "../core/services/INotificationService.js";
import { ILidoGovernanceMonitorLogger } from "../utils/logging/index.js";

export class NotificationService implements INotificationService {
  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly slackClient: ISlackClient,
    private readonly proposalRepository: IProposalRepository,
    private readonly riskThreshold: number,
    private readonly maxNotifyAttempts: number,
  ) {}

  async notifyOnce(): Promise<void> {
    try {
      this.logger.info("Starting notification processing");

      const analyzedProposals = await this.proposalRepository.findByStateForNotification(
        ProposalState.ANALYZED,
        this.maxNotifyAttempts,
      );
      const failedProposals = await this.proposalRepository.findByStateForNotification(
        ProposalState.NOTIFY_FAILED,
        this.maxNotifyAttempts,
      );
      const proposals = [...analyzedProposals, ...failedProposals];

      if (proposals.length === 0) {
        this.logger.debug("No proposals to notify");
        return;
      }

      this.logger.debug("Processing proposals for notification", { count: proposals.length });

      for (const proposal of proposals) {
        await this.notifyProposalInternal(proposal);
      }

      this.logger.info("Notification processing completed");
    } catch (error) {
      this.logger.critical("Notification processing failed", { error });
    }
  }

  private async notifyProposalInternal(proposal: ProposalWithoutText): Promise<void> {
    try {
      // Validate assessment exists
      if (!proposal.assessmentJson) {
        this.logger.error("Proposal missing assessment data", { proposalId: proposal.id });
        await this.proposalRepository.incrementNotifyAttempt(proposal.id);
        await this.proposalRepository.updateState(proposal.id, ProposalState.NOTIFY_FAILED);
        return;
      }

      const parseResult = AssessmentSchema.safeParse(proposal.assessmentJson);
      if (!parseResult.success) {
        this.logger.error("Proposal assessmentJson failed schema validation", {
          proposalId: proposal.id,
          errors: parseResult.error.errors,
        });
        await this.proposalRepository.incrementNotifyAttempt(proposal.id);
        await this.proposalRepository.updateState(proposal.id, ProposalState.NOTIFY_FAILED);
        return;
      }
      const assessment = parseResult.data;

      // Send to audit channel unconditionally
      const auditResult = await this.slackClient.sendAuditLog(proposal, assessment);
      if (!auditResult.success) {
        // Log but don't fail - audit is best-effort
        this.logger.critical("Audit log failed, continuing", {
          proposalId: proposal.id,
          error: auditResult.error,
        });
      }

      // Check risk threshold BEFORE attempting notification
      if (proposal.riskScore === null || proposal.riskScore < this.riskThreshold) {
        // Below threshold - mark as NOT_NOTIFIED without sending notification
        await this.proposalRepository.updateState(proposal.id, ProposalState.NOT_NOTIFIED);
        this.logger.info("Proposal below notification threshold, skipped", {
          proposalId: proposal.id,
          riskScore: proposal.riskScore,
          threshold: this.riskThreshold,
        });
        return;
      }

      // Increment attempt count before notification
      const updated = await this.proposalRepository.incrementNotifyAttempt(proposal.id);

      // Send Slack notification
      const result = await this.slackClient.sendProposalAlert(proposal, assessment);

      if (result.success) {
        await this.proposalRepository.markNotified(proposal.id);
        this.logger.info("Proposal notification sent", {
          proposalId: proposal.id,
        });
      } else {
        // Notification failed - transition to NOTIFY_FAILED for retry
        await this.proposalRepository.updateState(proposal.id, ProposalState.NOTIFY_FAILED);
        this.logger.warn("Slack notification failed, will retry", {
          proposalId: proposal.id,
          attempt: updated.notifyAttemptCount,
          error: result.error,
        });
        return;
      }
    } catch (error) {
      // Best-effort transition so the proposal doesn't silently drop out of the
      // notification pipeline when notifyAttemptCount was already incremented above.
      await this.proposalRepository.attemptUpdateState(proposal.id, ProposalState.NOTIFY_FAILED);
      this.logger.critical("Error notifying proposal", { proposalId: proposal.id, error });
    }
  }
}

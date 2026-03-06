import { jest, describe, it, expect, beforeEach, afterEach } from "@jest/globals";

import { IAIClient } from "../../core/clients/IAIClient.js";
import { Assessment, NativeYieldInvariant } from "../../core/entities/Assessment.js";
import { Proposal } from "../../core/entities/Proposal.js";
import { ProposalSource } from "../../core/entities/ProposalSource.js";
import { ProposalState } from "../../core/entities/ProposalState.js";
import { IProposalRepository } from "../../core/repositories/IProposalRepository.js";
import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";
import { ProposalProcessor } from "../ProposalProcessor.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  critical: jest.fn(),
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

describe("ProposalProcessor", () => {
  let processor: ProposalProcessor;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;
  let aiClient: jest.Mocked<IAIClient>;
  let proposalRepository: jest.Mocked<IProposalRepository>;

  const createMockProposal = (overrides: Partial<Proposal> = {}): Proposal => ({
    id: "uuid-1",
    source: ProposalSource.DISCOURSE,
    sourceId: "12345",
    url: "https://research.lido.fi/t/test/12345",
    title: "Test Proposal",
    author: "testuser",
    sourceCreatedAt: new Date("2024-01-15"),
    rawProposalText: "Proposal content for analysis",
    sourceMetadata: null,
    state: ProposalState.NEW,
    createdAt: new Date(),
    updatedAt: new Date(),
    stateUpdatedAt: new Date(),
    analysisAttemptCount: 0,
    llmModel: null,
    riskThreshold: null,
    assessmentPromptVersion: null,
    analyzedAt: null,
    assessmentJson: null,
    riskScore: null,
    notifyAttemptCount: 0,
    notifiedAt: null,
    ...overrides,
  });

  const createMockAssessment = (overrides: Partial<Assessment> = {}): Assessment => ({
    riskScore: 75,
    riskLevel: "high",
    confidence: 85,
    proposalType: "discourse",
    impactTypes: ["technical"],
    affectedComponents: ["StakingVault"],
    whatChanged: "Contract upgrade",
    nativeYieldInvariantsAtRisk: [NativeYieldInvariant.VALID_YIELD_REPORTING],
    nativeYieldImpact: ["May affect withdrawals"],
    recommendedAction: "escalate",
    urgency: "urgent",
    supportingQuotes: ["quote"],
    keyUnknowns: [],
    ...overrides,
  });

  beforeEach(() => {
    logger = createLoggerMock();
    aiClient = {
      analyzeProposal: jest.fn(),
      getModelName: jest.fn().mockReturnValue("claude-sonnet-4"),
    } as jest.Mocked<IAIClient>;
    proposalRepository = {
      findBySourceAndSourceId: jest.fn(),
      findByStateForAnalysis: jest.fn(),
      findByStateForNotification: jest.fn(),
      create: jest.fn(),
      upsert: jest.fn(),
      updateState: jest.fn(),
      attemptUpdateState: jest.fn(),
      saveAnalysis: jest.fn(),
      incrementAnalysisAttempt: jest.fn(),
      markNotified: jest.fn(),
      markNotifyFailed: jest.fn(),
      attemptMarkNotifyFailed: jest.fn(),
      findLatestSourceIdBySource: jest.fn(),
    } as jest.Mocked<IProposalRepository>;

    processor = new ProposalProcessor(
      logger,
      aiClient,
      proposalRepository,
      60, // riskThreshold
      "v1.0", // promptVersion
      5, // maxAnalysisAttempts
    );
  });

  afterEach(() => {});

  describe("processOnce", () => {
    it("fetches NEW and ANALYSIS_FAILED proposals from repository with max attempts filter", async () => {
      // Arrange
      proposalRepository.findByStateForAnalysis.mockResolvedValue([]);

      // Act
      await processor.processOnce();

      // Assert
      expect(proposalRepository.findByStateForAnalysis).toHaveBeenCalledWith(ProposalState.NEW, 5);
      expect(proposalRepository.findByStateForAnalysis).toHaveBeenCalledWith(ProposalState.ANALYSIS_FAILED, 5);
    });

    it("analyzes each NEW proposal with AI client", async () => {
      // Arrange
      const proposal = createMockProposal();
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([proposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      aiClient.analyzeProposal.mockResolvedValue(createMockAssessment());
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue(proposal);
      proposalRepository.saveAnalysis.mockResolvedValue(proposal);

      // Act
      await processor.processOnce();

      // Assert
      expect(aiClient.analyzeProposal).toHaveBeenCalledWith({
        proposalTitle: proposal.title,
        proposalText: proposal.rawProposalText,
        proposalUrl: proposal.url,
        proposalType: "discourse",
      });
    });

    it("saves analysis and transitions to ANALYZED state", async () => {
      // Arrange
      const proposal = createMockProposal();
      const assessment = createMockAssessment({ riskScore: 75 });
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([proposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      aiClient.analyzeProposal.mockResolvedValue(assessment);
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue(proposal);
      proposalRepository.saveAnalysis.mockResolvedValue({ ...proposal, state: ProposalState.ANALYZED });

      // Act
      await processor.processOnce();

      // Assert
      expect(proposalRepository.saveAnalysis).toHaveBeenCalledWith(
        proposal.id,
        assessment,
        75,
        "claude-sonnet-4",
        60,
        "v1.0",
      );
      // Should NOT call updateState anymore
      expect(proposalRepository.updateState).not.toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith("Proposal analysis completed", expect.any(Object));
    });

    it("increments attempt count and logs warning when AI analysis fails", async () => {
      // Arrange
      const proposal = createMockProposal({ analysisAttemptCount: 0 });
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([proposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      aiClient.analyzeProposal.mockResolvedValue(undefined);
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue({ ...proposal, analysisAttemptCount: 1 });

      // Act
      await processor.processOnce();

      // Assert
      expect(proposalRepository.incrementAnalysisAttempt).toHaveBeenCalledWith(proposal.id);
      expect(logger.warn).toHaveBeenCalledWith("AI analysis failed, will retry", expect.any(Object));
    });

    it("transitions proposal to ANALYSIS_FAILED when AI analysis returns undefined", async () => {
      // Arrange
      const newProposal = createMockProposal({ state: ProposalState.NEW });
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([newProposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue({ ...newProposal, analysisAttemptCount: 1 });
      aiClient.analyzeProposal.mockResolvedValue(undefined); // AI returns undefined
      proposalRepository.updateState.mockResolvedValue({ ...newProposal, state: ProposalState.ANALYSIS_FAILED });

      // Act
      await processor.processOnce();

      // Assert
      expect(proposalRepository.updateState).toHaveBeenCalledWith(newProposal.id, ProposalState.ANALYSIS_FAILED);
      expect(logger.warn).toHaveBeenCalledWith("AI analysis failed, will retry", expect.any(Object));
    });

    it("retries ANALYSIS_FAILED proposals", async () => {
      // Arrange
      const failedProposal = createMockProposal({
        state: ProposalState.ANALYSIS_FAILED,
        analysisAttemptCount: 3,
      });
      const assessment = createMockAssessment({ riskScore: 75 });
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([]) // NEW
        .mockResolvedValueOnce([failedProposal]); // ANALYSIS_FAILED
      aiClient.analyzeProposal.mockResolvedValue(assessment);
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue({ ...failedProposal, analysisAttemptCount: 4 });
      proposalRepository.saveAnalysis.mockResolvedValue({ ...failedProposal, state: ProposalState.ANALYZED });

      // Act
      await processor.processOnce();

      // Assert
      expect(aiClient.analyzeProposal).toHaveBeenCalled();
      expect(proposalRepository.saveAnalysis).toHaveBeenCalled();
      expect(proposalRepository.updateState).not.toHaveBeenCalled();
    });

    it("does nothing when no proposals need processing", async () => {
      // Arrange
      proposalRepository.findByStateForAnalysis.mockResolvedValue([]);

      // Act
      await processor.processOnce();

      // Assert
      expect(aiClient.analyzeProposal).not.toHaveBeenCalled();
      expect(logger.debug).toHaveBeenCalledWith("No proposals to process");
    });

    it("handles errors during processing gracefully and transitions to ANALYSIS_FAILED", async () => {
      // Arrange
      const proposal = createMockProposal();
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([proposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      proposalRepository.incrementAnalysisAttempt.mockRejectedValue(new Error("Database error"));
      proposalRepository.attemptUpdateState.mockResolvedValue(null);

      // Act
      await processor.processOnce();

      // Assert
      expect(proposalRepository.attemptUpdateState).toHaveBeenCalledWith(proposal.id, ProposalState.ANALYSIS_FAILED);
      expect(logger.critical).toHaveBeenCalledWith("Error processing proposal", expect.any(Object));
    });

    it("maps proposal source to proposalType correctly for snapshot", async () => {
      // Arrange
      const snapshotProposal = createMockProposal({ source: ProposalSource.SNAPSHOT });
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([snapshotProposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      aiClient.analyzeProposal.mockResolvedValue(createMockAssessment());
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue(snapshotProposal);
      proposalRepository.saveAnalysis.mockResolvedValue(snapshotProposal);

      // Act
      await processor.processOnce();

      // Assert
      expect(aiClient.analyzeProposal).toHaveBeenCalledWith(expect.objectContaining({ proposalType: "snapshot" }));
    });

    it("maps onchain voting contract sources to onchain_vote type", async () => {
      // Arrange
      const ldoProposal = createMockProposal({ source: ProposalSource.LDO_VOTING_CONTRACT });
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([ldoProposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      aiClient.analyzeProposal.mockResolvedValue(createMockAssessment());
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue(ldoProposal);
      proposalRepository.saveAnalysis.mockResolvedValue(ldoProposal);

      // Act
      await processor.processOnce();

      // Assert
      expect(aiClient.analyzeProposal).toHaveBeenCalledWith(expect.objectContaining({ proposalType: "onchain_vote" }));
    });

    it("catches and logs errors without throwing", async () => {
      // Arrange
      proposalRepository.findByStateForAnalysis.mockRejectedValue(new Error("Database connection error"));

      // Act
      await processor.processOnce();

      // Assert
      expect(logger.critical).toHaveBeenCalledWith("Proposal processing failed", expect.any(Object));
    });

    it("filters out proposals exceeding max analysis attempts at query level", async () => {
      // Arrange - DB returns no proposals because they all exceeded max attempts
      proposalRepository.findByStateForAnalysis.mockResolvedValue([]);

      // Act
      await processor.processOnce();

      // Assert - maxAnalysisAttempts (5) is passed to both queries
      expect(proposalRepository.findByStateForAnalysis).toHaveBeenCalledWith(ProposalState.NEW, 5);
      expect(proposalRepository.findByStateForAnalysis).toHaveBeenCalledWith(ProposalState.ANALYSIS_FAILED, 5);
      expect(aiClient.analyzeProposal).not.toHaveBeenCalled();
    });
  });
});

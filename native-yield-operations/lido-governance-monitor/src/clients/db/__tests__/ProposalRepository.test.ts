import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { PrismaClient } from "../../../../prisma/client/client.js";
import { ProposalSource } from "../../../core/entities/ProposalSource.js";
import { ProposalState } from "../../../core/entities/ProposalState.js";
import { ILidoGovernanceMonitorLogger } from "../../../utils/logging/index.js";
import { ProposalRepository } from "../ProposalRepository.js";

const mockPrisma = {
  proposal: {
    findUnique: jest.fn(),
    findFirst: jest.fn(),
    findMany: jest.fn(),
    create: jest.fn(),
    update: jest.fn(),
  },
};

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  critical: jest.fn(),
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

describe("ProposalRepository", () => {
  let repository: ProposalRepository;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;

  beforeEach(() => {
    jest.clearAllMocks();
    logger = createLoggerMock();
    repository = new ProposalRepository(logger, mockPrisma as unknown as PrismaClient);
  });

  describe("findBySourceAndSourceId", () => {
    it("returns proposal when found", async () => {
      // Arrange
      const mockProposal = { id: "uuid-1", source: "DISCOURSE", sourceId: "12345", state: "NEW" };
      mockPrisma.proposal.findUnique.mockResolvedValue(mockProposal);

      // Act
      const result = await repository.findBySourceAndSourceId(ProposalSource.DISCOURSE, "12345");

      // Assert
      expect(result).toEqual(mockProposal);
      expect(mockPrisma.proposal.findUnique).toHaveBeenCalledWith({
        where: { source_sourceId: { source: "DISCOURSE", sourceId: "12345" } },
      });
    });

    it("returns null when not found", async () => {
      // Arrange
      mockPrisma.proposal.findUnique.mockResolvedValue(null);

      // Act
      const result = await repository.findBySourceAndSourceId(ProposalSource.DISCOURSE, "99999");

      // Assert
      expect(result).toBeNull();
    });
  });

  describe("findByStateForAnalysis", () => {
    it("returns proposals with matching state ordered by stateUpdatedAt", async () => {
      // Arrange
      const mockProposals = [
        { id: "1", state: "NEW" },
        { id: "2", state: "NEW" },
      ];
      mockPrisma.proposal.findMany.mockResolvedValue(mockProposals);

      // Act
      const result = await repository.findByStateForAnalysis(ProposalState.NEW);

      // Assert
      expect(result).toEqual(mockProposals);
      expect(mockPrisma.proposal.findMany).toHaveBeenCalledWith({
        where: { state: "NEW" },
        orderBy: { stateUpdatedAt: "asc" },
      });
    });

    it("filters by analysisAttemptCount when maxAnalysisAttempts is provided", async () => {
      // Arrange
      mockPrisma.proposal.findMany.mockResolvedValue([]);

      // Act
      await repository.findByStateForAnalysis(ProposalState.ANALYSIS_FAILED, 5);

      // Assert
      expect(mockPrisma.proposal.findMany).toHaveBeenCalledWith({
        where: { state: "ANALYSIS_FAILED", analysisAttemptCount: { lt: 5 } },
        orderBy: { stateUpdatedAt: "asc" },
      });
    });
  });

  describe("findByStateForNotification", () => {
    it("returns proposals without text field ordered by stateUpdatedAt", async () => {
      // Arrange
      const mockProposals = [
        { id: "1", state: "ANALYZED" },
        { id: "2", state: "ANALYZED" },
      ];
      mockPrisma.proposal.findMany.mockResolvedValue(mockProposals);

      // Act
      const result = await repository.findByStateForNotification(ProposalState.ANALYZED);

      // Assert
      expect(result).toEqual(mockProposals);
      expect(mockPrisma.proposal.findMany).toHaveBeenCalledWith({
        where: { state: "ANALYZED" },
        orderBy: { stateUpdatedAt: "asc" },
        omit: { rawProposalText: true },
      });
    });

    it("filters by notifyAttemptCount when maxNotifyAttempts is provided", async () => {
      // Arrange
      mockPrisma.proposal.findMany.mockResolvedValue([]);

      // Act
      await repository.findByStateForNotification(ProposalState.NOTIFY_FAILED, 5);

      // Assert
      expect(mockPrisma.proposal.findMany).toHaveBeenCalledWith({
        where: { state: "NOTIFY_FAILED", notifyAttemptCount: { lt: 5 } },
        orderBy: { stateUpdatedAt: "asc" },
        omit: { rawProposalText: true },
      });
    });
  });

  describe("create", () => {
    it("creates a new proposal with NEW state", async () => {
      // Arrange
      const input = {
        source: ProposalSource.DISCOURSE,
        sourceId: "12345",
        url: "https://research.lido.fi/t/12345",
        title: "Test Proposal",
        author: "testuser",
        sourceCreatedAt: new Date("2024-01-15"),
        rawProposalText: "Proposal content",
      };
      mockPrisma.proposal.create.mockResolvedValue({ id: "new-uuid", ...input, state: "NEW" });

      // Act
      const result = await repository.create(input);

      // Assert
      expect(result.state).toBe("NEW");
      expect(mockPrisma.proposal.create).toHaveBeenCalled();
    });
  });

  describe("upsert", () => {
    const input = {
      source: ProposalSource.DISCOURSE,
      sourceId: "12345",
      url: "https://research.lido.fi/t/12345",
      title: "Test Proposal",
      author: "testuser",
      sourceCreatedAt: new Date("2024-01-15"),
      rawProposalText: "Proposal content",
    };

    it("returns existing proposal with isNew false when already exists", async () => {
      // Arrange
      const existing = { id: "existing-uuid", ...input, state: "NEW" };
      mockPrisma.proposal.findUnique.mockResolvedValue(existing);

      // Act
      const result = await repository.upsert(input);

      // Assert
      expect(result).toEqual({ proposal: existing, isNew: false });
      expect(mockPrisma.proposal.create).not.toHaveBeenCalled();
    });

    it("creates and returns new proposal with isNew true when not found", async () => {
      // Arrange
      const created = { id: "new-uuid", ...input, state: "NEW" };
      mockPrisma.proposal.findUnique.mockResolvedValue(null);
      mockPrisma.proposal.create.mockResolvedValue(created);

      // Act
      const result = await repository.upsert(input);

      // Assert
      expect(result).toEqual({ proposal: created, isNew: true });
      expect(mockPrisma.proposal.create).toHaveBeenCalled();
    });
  });

  describe("updateState", () => {
    it("updates proposal state and stateUpdatedAt", async () => {
      // Arrange
      mockPrisma.proposal.update.mockResolvedValue({ id: "uuid-1", state: "ANALYZED" });

      // Act
      const result = await repository.updateState("uuid-1", ProposalState.ANALYZED);

      // Assert
      expect(result.state).toBe("ANALYZED");
      expect(mockPrisma.proposal.update).toHaveBeenCalledWith({
        where: { id: "uuid-1" },
        data: { state: "ANALYZED", stateUpdatedAt: expect.any(Date) },
      });
    });
  });

  describe("saveAnalysis", () => {
    it("saves assessment and transitions to ANALYZED state", async () => {
      // Arrange
      const assessment = {
        riskScore: 75,
        impactType: "technical" as const,
        riskLevel: "high" as const,
        whatChanged: "Contract upgrade",
        nativeYieldImpact: ["May affect withdrawals"],
        recommendedAction: "escalate" as const,
        supportingQuotes: ["quote1"],
      };
      mockPrisma.proposal.update.mockResolvedValue({ id: "uuid-1", state: "ANALYZED" });

      // Act
      await repository.saveAnalysis("uuid-1", assessment, 75, "claude-sonnet-4", 60, "v1.0");

      // Assert
      expect(mockPrisma.proposal.update).toHaveBeenCalledWith({
        where: { id: "uuid-1" },
        data: expect.objectContaining({
          state: "ANALYZED",
          assessmentJson: assessment,
          riskScore: 75,
          llmModel: "claude-sonnet-4",
        }),
      });
    });
  });

  describe("incrementAnalysisAttempt", () => {
    it("increments the analysis attempt count", async () => {
      // Arrange
      mockPrisma.proposal.update.mockResolvedValue({ id: "uuid-1", analysisAttemptCount: 2 });

      // Act
      await repository.incrementAnalysisAttempt("uuid-1");

      // Assert
      expect(mockPrisma.proposal.update).toHaveBeenCalledWith({
        where: { id: "uuid-1" },
        data: { analysisAttemptCount: { increment: 1 } },
      });
    });
  });

  describe("incrementNotifyAttempt", () => {
    it("increments the notify attempt count", async () => {
      // Arrange
      mockPrisma.proposal.update.mockResolvedValue({ id: "uuid-1", notifyAttemptCount: 2 });

      // Act
      await repository.incrementNotifyAttempt("uuid-1");

      // Assert
      expect(mockPrisma.proposal.update).toHaveBeenCalledWith({
        where: { id: "uuid-1" },
        data: { notifyAttemptCount: { increment: 1 } },
      });
    });
  });

  describe("findLatestSourceIdBySource", () => {
    it("returns sourceId when proposal exists for source", async () => {
      // Arrange
      mockPrisma.proposal.findFirst.mockResolvedValue({ sourceId: "180" });

      // Act
      const result = await repository.findLatestSourceIdBySource(ProposalSource.LDO_VOTING_CONTRACT);

      // Assert
      expect(result).toBe("180");
      expect(mockPrisma.proposal.findFirst).toHaveBeenCalledWith({
        where: { source: "LDO_VOTING_CONTRACT" },
        orderBy: { sourceCreatedAt: "desc" },
        select: { sourceId: true },
      });
    });

    it("returns null when no proposal exists for source", async () => {
      // Arrange
      mockPrisma.proposal.findFirst.mockResolvedValue(null);

      // Act
      const result = await repository.findLatestSourceIdBySource(ProposalSource.DISCOURSE);

      // Assert
      expect(result).toBeNull();
    });
  });

  describe("attemptUpdateState", () => {
    it("returns updated proposal on success", async () => {
      // Arrange
      const updated = { id: "uuid-1", state: "ANALYSIS_FAILED" };
      mockPrisma.proposal.update.mockResolvedValue(updated);

      // Act
      const result = await repository.attemptUpdateState("uuid-1", ProposalState.ANALYSIS_FAILED);

      // Assert
      expect(result).toEqual(updated);
      expect(mockPrisma.proposal.update).toHaveBeenCalledWith({
        where: { id: "uuid-1" },
        data: { state: "ANALYSIS_FAILED", stateUpdatedAt: expect.any(Date) },
      });
    });

    it("returns null and logs critical when updateState throws", async () => {
      // Arrange
      mockPrisma.proposal.update.mockRejectedValue(new Error("Database error"));

      // Act
      const result = await repository.attemptUpdateState("uuid-1", ProposalState.NOTIFY_FAILED);

      // Assert
      expect(result).toBeNull();
      expect(logger.critical).toHaveBeenCalledWith("attemptUpdateState failed", {
        id: "uuid-1",
        state: ProposalState.NOTIFY_FAILED,
        error: expect.any(Error),
      });
    });
  });

  describe("markNotified", () => {
    it("marks proposal as NOTIFIED with notifiedAt timestamp", async () => {
      // Arrange
      mockPrisma.proposal.update.mockResolvedValue({ id: "uuid-1", state: "NOTIFIED" });

      // Act
      await repository.markNotified("uuid-1");

      // Assert
      expect(mockPrisma.proposal.update).toHaveBeenCalledWith({
        where: { id: "uuid-1" },
        data: expect.objectContaining({
          state: "NOTIFIED",
          notifiedAt: expect.any(Date),
        }),
      });
    });
  });
});

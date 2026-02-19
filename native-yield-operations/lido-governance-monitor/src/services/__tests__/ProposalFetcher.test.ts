import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { CreateProposalInput } from "../../core/entities/Proposal.js";
import { ProposalSource } from "../../core/entities/ProposalSource.js";
import { IProposalFetcher } from "../../core/services/IProposalFetcher.js";
import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";
import { ProposalFetcher } from "../ProposalFetcher.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  critical: jest.fn(),
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

const createSourceFetcherMock = (): jest.Mocked<IProposalFetcher> => ({
  getLatestProposals: jest.fn(),
});

const createProposalInput = (overrides: Partial<CreateProposalInput> = {}): CreateProposalInput => ({
  source: ProposalSource.DISCOURSE,
  sourceId: "100",
  url: "https://research.lido.fi/t/proposal/100",
  title: "Proposal 100",
  author: "author",
  sourceCreatedAt: new Date("2024-01-15"),
  rawProposalText: "Content",
  ...overrides,
});

describe("ProposalFetcher", () => {
  let fetcher: ProposalFetcher;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;
  let sourceFetcherA: jest.Mocked<IProposalFetcher>;
  let sourceFetcherB: jest.Mocked<IProposalFetcher>;

  beforeEach(() => {
    logger = createLoggerMock();
    sourceFetcherA = createSourceFetcherMock();
    sourceFetcherB = createSourceFetcherMock();

    fetcher = new ProposalFetcher(logger, [sourceFetcherA, sourceFetcherB]);
  });

  describe("getLatestProposals", () => {
    it("calls getLatestProposals on all fetchers", async () => {
      // Arrange
      sourceFetcherA.getLatestProposals.mockResolvedValue([]);
      sourceFetcherB.getLatestProposals.mockResolvedValue([]);

      // Act
      await fetcher.getLatestProposals();

      // Assert
      expect(sourceFetcherA.getLatestProposals).toHaveBeenCalled();
      expect(sourceFetcherB.getLatestProposals).toHaveBeenCalled();
    });

    it("continues when one fetcher rejects", async () => {
      // Arrange
      const proposal = createProposalInput();
      sourceFetcherA.getLatestProposals.mockRejectedValue(new Error("Network error"));
      sourceFetcherB.getLatestProposals.mockResolvedValue([proposal]);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(logger.critical).toHaveBeenCalledWith(
        "Source fetcher failed",
        expect.objectContaining({ error: expect.any(Error) }),
      );
      expect(result).toEqual([proposal]);
    });

    it("works with empty fetcher array", async () => {
      // Arrange
      const emptyFetcher = new ProposalFetcher(logger, []);

      // Act
      await emptyFetcher.getLatestProposals();

      // Assert
      expect(logger.info).toHaveBeenCalledWith("Proposal polling completed", { sources: 0, fetched: 0 });
    });

    it("works when all fetchers return empty arrays", async () => {
      // Arrange
      sourceFetcherA.getLatestProposals.mockResolvedValue([]);
      sourceFetcherB.getLatestProposals.mockResolvedValue([]);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toEqual([]);
      expect(logger.info).toHaveBeenCalledWith("Proposal polling completed", { sources: 2, fetched: 0 });
    });
  });
});

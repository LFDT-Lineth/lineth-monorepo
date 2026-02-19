import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { ProposalSource } from "../../../core/entities/ProposalSource.js";
import { IProposalRepository } from "../../../core/repositories/IProposalRepository.js";
import { ILidoGovernanceMonitorLogger } from "../../../utils/logging/index.js";
import { LdoVotingContractFetcher } from "../../fetchers/LdoVotingContractFetcher.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  critical: jest.fn(),
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

interface MockPublicClient {
  getLogs: jest.Mock;
  readContract: jest.Mock;
}

const createPublicClientMock = (): MockPublicClient => ({
  getLogs: jest.fn(),
  readContract: jest.fn(),
});

const createProposalRepositoryMock = (): jest.Mocked<
  Pick<IProposalRepository, "findLatestSourceIdBySource" | "create" | "upsert" | "findBySourceAndSourceId">
> => ({
  findLatestSourceIdBySource: jest.fn(),
  create: jest.fn(),
  upsert: jest.fn(),
  findBySourceAndSourceId: jest.fn(),
});

describe("LdoVotingContractFetcher", () => {
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;
  let publicClient: MockPublicClient;
  let proposalRepository: jest.Mocked<
    Pick<IProposalRepository, "findLatestSourceIdBySource" | "create" | "upsert" | "findBySourceAndSourceId">
  >;
  const contractAddress = "0x2e59a20f205bb85a89c53f1936454680651e618e";
  const initialVoteId = 150n;

  beforeEach(() => {
    logger = createLoggerMock();
    publicClient = createPublicClientMock();
    proposalRepository = createProposalRepositoryMock();
    proposalRepository.upsert.mockResolvedValue({ proposal: { id: "uuid" } as never, isNew: true });
  });

  const createFetcher = (overrideInitialVoteId?: bigint | undefined): LdoVotingContractFetcher =>
    new LdoVotingContractFetcher(
      logger,
      publicClient as never,
      contractAddress,
      overrideInitialVoteId !== undefined ? overrideInitialVoteId : initialVoteId,
      proposalRepository as unknown as IProposalRepository,
    );

  describe("getLatestProposals", () => {
    it("returns empty array when no new votes (DB voteId equals votesLength - 1)", async () => {
      // Arrange
      const fetcher = createFetcher();
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue("180");
      publicClient.readContract.mockResolvedValue(181n); // votesLength

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toEqual([]);
      expect(publicClient.getLogs).not.toHaveBeenCalled();
      // readContract should only be called once for votesLength, no getVote calls
      expect(publicClient.readContract).toHaveBeenCalledTimes(1);
    });

    it("fetches single new vote when DB is one behind", async () => {
      // Arrange
      const fetcher = createFetcher();
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue("179");
      // First readContract: votesLength
      publicClient.readContract.mockResolvedValueOnce(181n);
      // Second readContract: getVote(180)
      publicClient.readContract.mockResolvedValueOnce([
        false, // open
        false, // executed
        1700000000n, // startDate
        5000n, // snapshotBlock
        500000000000000000n, // supportRequired
        500000000000000000n, // minAcceptQuorum
        1000n, // yea
        500n, // nay
        2000n, // votingPower
        "0x", // script
        0, // phase
      ]);
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 180n, creator: "0xabc", metadata: "vote text" } }]);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        source: ProposalSource.LDO_VOTING_CONTRACT,
        sourceId: "180",
        url: "https://vote.lido.fi/vote/180",
        title: "LDO Contract vote 180",
        author: "0xabc",
        sourceCreatedAt: new Date(1700000000 * 1000),
        rawProposalText: "vote text",
      });
      expect(publicClient.getLogs).toHaveBeenCalledWith(
        expect.objectContaining({
          args: { voteId: 180n },
          fromBlock: 5000n,
          toBlock: 5009n,
        }),
      );
    });

    it("starts from initialVoteId when DB has no entries", async () => {
      // Arrange
      const fetcher = createFetcher();
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue(null);
      // votesLength
      publicClient.readContract.mockResolvedValueOnce(152n);
      // getVote(150)
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000000n, 5000n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      // getVote(151)
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000100n, 5100n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      // getLogs for 150
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 150n, creator: "0xaaa", metadata: "vote 150" } }]);
      // getLogs for 151
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 151n, creator: "0xbbb", metadata: "vote 151" } }]);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toHaveLength(2);
      expect(result[0].sourceId).toBe("150");
      expect(result[1].sourceId).toBe("151");
    });

    it("returns empty array when votesLength is 0", async () => {
      // Arrange
      const fetcher = createFetcher();
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue(null);
      publicClient.readContract.mockResolvedValueOnce(0n); // votesLength

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toEqual([]);
      expect(publicClient.getLogs).not.toHaveBeenCalled();
    });

    it("stops fetching at first failure and returns only votes before the gap", async () => {
      // Arrange
      const fetcher = createFetcher(10n);
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue(null);
      // votesLength
      publicClient.readContract.mockResolvedValueOnce(13n);
      // getVote(10) - success
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000000n, 5000n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      // getVote(11) - fails
      publicClient.readContract.mockRejectedValueOnce(new Error("RPC timeout"));
      // getVote(12) - should never be called
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000200n, 5200n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      // getLogs for 10
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 10n, creator: "0xaaa", metadata: "vote 10" } }]);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert - only vote 10, not vote 12
      expect(result).toHaveLength(1);
      expect(result[0].sourceId).toBe("10");
      // readContract called 3 times: votesLength + getVote(10) + getVote(11 which failed)
      expect(publicClient.readContract).toHaveBeenCalledTimes(3);
      // getLogs only called for vote 10, not vote 12
      expect(publicClient.getLogs).toHaveBeenCalledTimes(1);
      expect(logger.critical).toHaveBeenCalledWith(
        expect.stringContaining("11"),
        expect.objectContaining({ error: "RPC timeout" }),
      );
    });

    it("stops and returns no further proposals when DB persistence fails", async () => {
      // Arrange
      const fetcher = createFetcher(10n);
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue(null);
      // votesLength
      publicClient.readContract.mockResolvedValueOnce(13n);
      // getVote(10) - success
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000000n, 5000n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      // getVote(11) - success (RPC works)
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000100n, 5100n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      // getLogs for 10
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 10n, creator: "0xaaa", metadata: "vote 10" } }]);
      // getLogs for 11
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 11n, creator: "0xbbb", metadata: "vote 11" } }]);

      // upsert(vote 10) succeeds, upsert(vote 11) fails
      proposalRepository.upsert.mockResolvedValueOnce({ proposal: { id: "uuid-10" } as never, isNew: true });
      proposalRepository.upsert.mockRejectedValueOnce(new Error("DB connection lost"));

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert - only vote 10 returned, vote 11 failed to persist, vote 12 never fetched
      expect(result).toHaveLength(1);
      expect(result[0].sourceId).toBe("10");
      expect(proposalRepository.upsert).toHaveBeenCalledTimes(2);
      // readContract: votesLength + getVote(10) + getVote(11) = 3 (vote 12 never called)
      expect(publicClient.readContract).toHaveBeenCalledTimes(3);
      expect(logger.critical).toHaveBeenCalledWith(
        expect.stringContaining("11"),
        expect.objectContaining({ error: "DB connection lost" }),
      );
    });

    it("creates proposal with null author and empty text when getLogs returns empty", async () => {
      // Arrange
      const fetcher = createFetcher();
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue("179");
      publicClient.readContract.mockResolvedValueOnce(181n); // votesLength
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000000n, 5000n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      publicClient.getLogs.mockResolvedValueOnce([]); // empty logs

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toHaveLength(1);
      expect(result[0].author).toBeNull();
      expect(result[0].rawProposalText).toBe("");
    });

    it("returns empty array and logs critical when votesLength call fails", async () => {
      // Arrange
      const fetcher = createFetcher();
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue(null);
      publicClient.readContract.mockRejectedValueOnce(new Error("RPC error"));

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toEqual([]);
      expect(logger.critical).toHaveBeenCalledWith(
        expect.stringContaining("votesLength"),
        expect.objectContaining({ error: "RPC error" }),
      );
    });

    it("uses startDate from getVote for sourceCreatedAt (not getBlock)", async () => {
      // Arrange
      const fetcher = createFetcher();
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue("179");
      publicClient.readContract.mockResolvedValueOnce(181n); // votesLength
      const startDate = 1609459200n; // 2021-01-01T00:00:00Z
      publicClient.readContract.mockResolvedValueOnce([false, false, startDate, 5000n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 180n, creator: "0xabc", metadata: "text" } }]);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result[0].sourceCreatedAt).toEqual(new Date(Number(startDate) * 1000));
      expect(result[0].sourceCreatedAt).toEqual(new Date("2021-01-01T00:00:00.000Z"));
    });

    it("calls getLogs with voteId filter and narrow 10-block window", async () => {
      // Arrange
      const fetcher = createFetcher(42n);
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue(null);
      publicClient.readContract.mockResolvedValueOnce(43n); // votesLength
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000000n, 5000n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 42n, creator: "0xabc", metadata: "text" } }]);

      // Act
      await fetcher.getLatestProposals();

      // Assert
      expect(publicClient.getLogs).toHaveBeenCalledWith(
        expect.objectContaining({
          args: { voteId: 42n },
          fromBlock: 5000n,
          toBlock: 5009n,
        }),
      );
    });

    it("fetches multiple new votes in correct order", async () => {
      // Arrange
      const fetcher = createFetcher();
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue("5");
      publicClient.readContract.mockResolvedValueOnce(8n); // votesLength
      // getVote(6)
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000000n, 5000n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      // getVote(7)
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000100n, 5100n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 6n, creator: "0xaaa", metadata: "vote 6" } }]);
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 7n, creator: "0xbbb", metadata: "vote 7" } }]);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toHaveLength(2);
      expect(result[0].sourceId).toBe("6");
      expect(result[1].sourceId).toBe("7");
    });

    it("starts from voteId 0 when DB has no entries and initialVoteId is undefined", async () => {
      // Arrange
      const fetcher = new LdoVotingContractFetcher(
        logger,
        publicClient as never,
        contractAddress,
        undefined,
        proposalRepository as unknown as IProposalRepository,
      );
      proposalRepository.findLatestSourceIdBySource.mockResolvedValue(null);
      publicClient.readContract.mockResolvedValueOnce(2n); // votesLength
      // getVote(0)
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000000n, 5000n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      // getVote(1)
      publicClient.readContract.mockResolvedValueOnce([false, false, 1700000100n, 5100n, 0n, 0n, 0n, 0n, 0n, "0x", 0]);
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 0n, creator: "0xaaa", metadata: "vote 0" } }]);
      publicClient.getLogs.mockResolvedValueOnce([{ args: { voteId: 1n, creator: "0xbbb", metadata: "vote 1" } }]);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toHaveLength(2);
      expect(result[0].sourceId).toBe("0");
      expect(result[1].sourceId).toBe("1");
    });
  });
});

import { Address } from "viem";

import { RebalanceDirection } from "../../core/entities/RebalanceRequirement.js";
import { createLoggerMock, createMetricsUpdaterMock } from "../../__tests__/helpers/index.js";
import { RebalanceQuotaService } from "../RebalanceQuotaService.js";

describe("RebalanceQuotaService", () => {
  let logger: ReturnType<typeof createLoggerMock>;
  let metricsUpdater: ReturnType<typeof createMetricsUpdaterMock>;
  let service: RebalanceQuotaService;

  const VAULT_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
  const TOTAL_SYSTEM_BALANCE_WEI = 1000000000000000000000n; // 1000 ETH
  const REBALANCE_QUOTA_BPS = 1800; // 18%
  const REBALANCE_TOLERANCE_WEI = 1000000000000000000n; // 1 ETH
  const QUOTA_WINDOW_SIZE_CYCLES = 24;

  const createService = (
    stakingDirection: RebalanceDirection = RebalanceDirection.STAKE,
    windowSize: number = QUOTA_WINDOW_SIZE_CYCLES,
    quotaBps: number = REBALANCE_QUOTA_BPS,
    toleranceWei: bigint = REBALANCE_TOLERANCE_WEI,
  ) => {
    return new RebalanceQuotaService(logger, metricsUpdater, stakingDirection, windowSize, quotaBps, toleranceWei);
  };

  beforeEach(() => {
    logger = createLoggerMock();
    metricsUpdater = createMetricsUpdaterMock();
  });

  describe("constructor", () => {
    it("initializes service with provided parameters", () => {
      // Arrange
      const direction = RebalanceDirection.STAKE;

      // Act
      service = createService(direction);

      // Assert
      expect(service).toBeDefined();
      expect(service.getStakingDirection()).toBe(direction);
    });

    it("logs initialization message with configuration details", () => {
      // Arrange
      const direction = RebalanceDirection.STAKE;

      // Act
      service = createService(direction);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("RebalanceQuotaService initialized"));
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining(`stakingDirection=${direction}`));
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`quotaWindowSizeInCycles=${QUOTA_WINDOW_SIZE_CYCLES}`),
      );
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining(`rebalanceQuotaBps=${REBALANCE_QUOTA_BPS}`));
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`rebalanceToleranceAmountWei=${REBALANCE_TOLERANCE_WEI.toString()}`),
      );
    });

    it("stores staking direction for unstake operations", () => {
      // Arrange
      const direction = RebalanceDirection.UNSTAKE;
      const customWindowSize = 48;
      const customQuotaBps = 2000;
      const customTolerance = 2000000000000000000n;

      // Act
      service = new RebalanceQuotaService(
        logger,
        metricsUpdater,
        direction,
        customWindowSize,
        customQuotaBps,
        customTolerance,
      );

      // Assert
      expect(service.getStakingDirection()).toBe(direction);
    });

    it("creates sliding window accumulator with configured size", () => {
      // Arrange
      const customWindowSize = 12;
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const amountPerCycle = quotaWei / BigInt(customWindowSize);

      // Act
      service = createService(RebalanceDirection.STAKE, customWindowSize);

      // Fill the window completely
      for (let i = 0; i < customWindowSize; i++) {
        service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, amountPerCycle);
      }

      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        amountPerCycle,
      );

      // Assert
      expect(result).toBeGreaterThan(0n);
    });
  });

  describe("getStakingDirection", () => {
    it("returns STAKE direction configured in constructor", () => {
      // Arrange
      service = createService(RebalanceDirection.STAKE);

      // Act
      const result = service.getStakingDirection();

      // Assert
      expect(result).toBe(RebalanceDirection.STAKE);
    });

    it("returns UNSTAKE direction configured in constructor", () => {
      // Arrange
      service = createService(RebalanceDirection.UNSTAKE);

      // Act
      const result = service.getStakingDirection();

      // Assert
      expect(result).toBe(RebalanceDirection.UNSTAKE);
    });
  });

  describe("getRebalanceAmountAfterQuota - quota disabled", () => {
    it("returns full requested amount when window size is zero", () => {
      // Arrange
      const DISABLED_WINDOW_SIZE = 0;
      const REBALANCE_AMOUNT_WEI = 50000000000000000000n; // 50 ETH
      service = createService(RebalanceDirection.STAKE, DISABLED_WINDOW_SIZE);

      // Act
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        REBALANCE_AMOUNT_WEI,
      );

      // Assert
      expect(result).toBe(REBALANCE_AMOUNT_WEI);
    });

    it("skips metrics when quota enforcement is disabled", () => {
      // Arrange
      const DISABLED_WINDOW_SIZE = 0;
      const REBALANCE_AMOUNT_WEI = 50000000000000000000n;
      service = createService(RebalanceDirection.STAKE, DISABLED_WINDOW_SIZE);

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, REBALANCE_AMOUNT_WEI);

      // Assert
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).not.toHaveBeenCalled();
    });

    it("skips quota logging when enforcement is disabled", () => {
      // Arrange
      const DISABLED_WINDOW_SIZE = 0;
      const REBALANCE_AMOUNT_WEI = 50000000000000000000n;
      service = createService(RebalanceDirection.STAKE, DISABLED_WINDOW_SIZE);

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, REBALANCE_AMOUNT_WEI);

      // Assert
      expect(logger.info).not.toHaveBeenCalledWith(expect.stringContaining("below tolerance threshold"));
      expect(logger.info).not.toHaveBeenCalledWith(expect.stringContaining("quotaWei"));
    });
  });

  describe("getRebalanceAmountAfterQuota - below tolerance threshold", () => {
    it("returns zero when amount is below tolerance threshold", () => {
      // Arrange
      const BELOW_THRESHOLD_AMOUNT_WEI = 500000000000000000n; // 0.5 ETH, below 1 ETH threshold
      service = createService();

      // Act
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        BELOW_THRESHOLD_AMOUNT_WEI,
      );

      // Assert
      expect(result).toBe(0n);
    });

    it("tracks zero in window buffer instead of actual amount", () => {
      // Arrange
      const BELOW_THRESHOLD_AMOUNT_WEI = 500000000000000000n; // 0.5 ETH
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const HALF_QUOTA_AMOUNT_WEI = quotaWei / 2n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, BELOW_THRESHOLD_AMOUNT_WEI);
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        HALF_QUOTA_AMOUNT_WEI,
      );

      // Assert
      expect(result).toBe(HALF_QUOTA_AMOUNT_WEI);
    });

    it("logs threshold rejection with amount details", () => {
      // Arrange
      const BELOW_THRESHOLD_AMOUNT_WEI = 500000000000000000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, BELOW_THRESHOLD_AMOUNT_WEI);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("below tolerance threshold"));
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`reBalanceAmountWei=${BELOW_THRESHOLD_AMOUNT_WEI.toString()}`),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`rebalanceToleranceAmountWei=${REBALANCE_TOLERANCE_WEI.toString()}`),
      );
    });

    it("skips metrics when amount is below threshold", () => {
      // Arrange
      const BELOW_THRESHOLD_AMOUNT_WEI = 500000000000000000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, BELOW_THRESHOLD_AMOUNT_WEI);

      // Assert
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).not.toHaveBeenCalled();
    });
  });

  describe("getRebalanceAmountAfterQuota - within quota (happy path)", () => {
    it("returns full amount when request is within quota", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const HALF_QUOTA_AMOUNT_WEI = quotaWei / 2n;
      service = createService();

      // Act
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        HALF_QUOTA_AMOUNT_WEI,
      );

      // Assert
      expect(result).toBe(HALF_QUOTA_AMOUNT_WEI);
    });

    it("calculates quota using basis points formula", () => {
      // Arrange
      const EXPECTED_QUOTA_WEI = 180000000000000000000n; // 1000 ETH * 1800 bps / 10000 = 180 ETH
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const JUST_UNDER_QUOTA_WEI = quotaWei - 1n;
      service = createService();

      // Act
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        JUST_UNDER_QUOTA_WEI,
      );

      // Assert
      expect(quotaWei).toBe(EXPECTED_QUOTA_WEI);
      expect(result).toBe(JUST_UNDER_QUOTA_WEI);
    });

    it("accumulates amounts in window buffer across cycles", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const THIRD_QUOTA_AMOUNT_WEI = quotaWei / 3n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, THIRD_QUOTA_AMOUNT_WEI);
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, THIRD_QUOTA_AMOUNT_WEI);
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        THIRD_QUOTA_AMOUNT_WEI,
      );

      // Assert
      expect(result).toBe(THIRD_QUOTA_AMOUNT_WEI);
    });

    it("logs quota calculation with system state", () => {
      // Arrange
      const REBALANCE_AMOUNT_WEI = 50000000000000000000n; // 50 ETH
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, REBALANCE_AMOUNT_WEI);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`totalSystemBalance=${TOTAL_SYSTEM_BALANCE_WEI.toString()}`),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`reBalanceAmountWei=${REBALANCE_AMOUNT_WEI.toString()}`),
      );
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining(`quotaWei=${quotaWei.toString()}`));
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("prevTotal"));
    });

    it("skips metrics when quota is not exceeded", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const HALF_QUOTA_AMOUNT_WEI = quotaWei / 2n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, HALF_QUOTA_AMOUNT_WEI);

      // Assert
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).not.toHaveBeenCalled();
    });
  });

  describe("getRebalanceAmountAfterQuota - exceeding quota scenarios", () => {
    it("returns partial amount when request would exceed quota", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const NINETY_FIVE_PCT_QUOTA_WEI = (quotaWei * 95n) / 100n;
      const TEN_PCT_QUOTA_WEI = quotaWei / 10n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, NINETY_FIVE_PCT_QUOTA_WEI);
      const result = service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, TEN_PCT_QUOTA_WEI);

      // Assert
      const expectedPartial = quotaWei - NINETY_FIVE_PCT_QUOTA_WEI;
      expect(result).toBe(expectedPartial);
      expect(result).toBeLessThan(TEN_PCT_QUOTA_WEI);
    });

    it("returns zero when quota already exceeded in previous cycle", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const EXCESS_AMOUNT_WEI = quotaWei + 1000000000000000000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, EXCESS_AMOUNT_WEI);
      const result = service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, EXCESS_AMOUNT_WEI);

      // Assert
      expect(result).toBe(0n);
    });

    it("increments quota exceeded metric when returning partial amount", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const NINETY_FIVE_PCT_QUOTA_WEI = (quotaWei * 95n) / 100n;
      const TEN_PCT_QUOTA_WEI = quotaWei / 10n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, NINETY_FIVE_PCT_QUOTA_WEI);
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, TEN_PCT_QUOTA_WEI);

      // Assert
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).toHaveBeenCalledWith(VAULT_ADDRESS);
    });

    it("increments quota exceeded metric when returning zero", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const EXCESS_AMOUNT_WEI = quotaWei + 1000000000000000000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, EXCESS_AMOUNT_WEI);
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, EXCESS_AMOUNT_WEI);

      // Assert
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).toHaveBeenCalledTimes(2);
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).toHaveBeenCalledWith(VAULT_ADDRESS);
    });

    it("logs new total when quota is exceeded", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const NINETY_FIVE_PCT_QUOTA_WEI = (quotaWei * 95n) / 100n;
      const TEN_PCT_QUOTA_WEI = quotaWei / 10n;
      service = createService();
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, NINETY_FIVE_PCT_QUOTA_WEI);

      // Act
      jest.clearAllMocks();
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, TEN_PCT_QUOTA_WEI);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("newTotal"));
    });
  });

  describe("edge cases and integration", () => {
    it("returns zero when system balance is zero", () => {
      // Arrange
      const ZERO_BALANCE_WEI = 0n;
      const REBALANCE_AMOUNT_WEI = 1000000000000000000n;
      service = createService();

      // Act
      const result = service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, ZERO_BALANCE_WEI, REBALANCE_AMOUNT_WEI);

      // Assert
      expect(result).toBe(0n);
    });

    it("handles very large balance values", () => {
      // Arrange
      const VERY_LARGE_BALANCE_WEI = 1000000000000000000000000n; // 1 million ETH
      const quotaWei = (VERY_LARGE_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const HALF_QUOTA_AMOUNT_WEI = quotaWei / 2n;
      service = createService();

      // Act
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        VERY_LARGE_BALANCE_WEI,
        HALF_QUOTA_AMOUNT_WEI,
      );

      // Assert
      expect(result).toBe(HALF_QUOTA_AMOUNT_WEI);
    });

    it("maintains sliding window across multiple cycles", () => {
      // Arrange
      const SMALL_WINDOW_SIZE = 3;
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const QUARTER_QUOTA_AMOUNT_WEI = quotaWei / 4n;
      service = createService(RebalanceDirection.STAKE, SMALL_WINDOW_SIZE);

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, QUARTER_QUOTA_AMOUNT_WEI);
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, QUARTER_QUOTA_AMOUNT_WEI);
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, QUARTER_QUOTA_AMOUNT_WEI);
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        QUARTER_QUOTA_AMOUNT_WEI,
      );

      // Assert
      expect(result).toBe(QUARTER_QUOTA_AMOUNT_WEI);
    });

    it("evicts oldest value when window slides", () => {
      // Arrange
      const TWO_CYCLE_WINDOW = 2;
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const THIRD_QUOTA_AMOUNT_WEI = quotaWei / 3n;
      service = createService(RebalanceDirection.STAKE, TWO_CYCLE_WINDOW);

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, THIRD_QUOTA_AMOUNT_WEI);
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, THIRD_QUOTA_AMOUNT_WEI);
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        THIRD_QUOTA_AMOUNT_WEI,
      );

      // Assert
      expect(result).toBe(THIRD_QUOTA_AMOUNT_WEI);
    });

    it("calculates quota with low basis points percentage", () => {
      // Arrange
      const LOW_BPS = 100; // 1%
      const EXPECTED_QUOTA_WEI = 10000000000000000000n; // 10 ETH (1% of 1000 ETH)
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * 100n) / 10000n;
      const HALF_QUOTA_AMOUNT_WEI = quotaWei / 2n;
      service = createService(RebalanceDirection.STAKE, QUOTA_WINDOW_SIZE_CYCLES, LOW_BPS);

      // Act
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        HALF_QUOTA_AMOUNT_WEI,
      );

      // Assert
      expect(quotaWei).toBe(EXPECTED_QUOTA_WEI);
      expect(result).toBe(HALF_QUOTA_AMOUNT_WEI);
    });

    it("calculates quota with high basis points percentage", () => {
      // Arrange
      const HIGH_BPS = 1800; // 18%
      const EXPECTED_QUOTA_WEI = 180000000000000000000n; // 180 ETH (18% of 1000 ETH)
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * 1800n) / 10000n;
      const HALF_QUOTA_AMOUNT_WEI = quotaWei / 2n;
      service = createService(RebalanceDirection.STAKE, QUOTA_WINDOW_SIZE_CYCLES, HIGH_BPS);

      // Act
      const result = service.getRebalanceAmountAfterQuota(
        VAULT_ADDRESS,
        TOTAL_SYSTEM_BALANCE_WEI,
        HALF_QUOTA_AMOUNT_WEI,
      );

      // Assert
      expect(quotaWei).toBe(EXPECTED_QUOTA_WEI);
      expect(result).toBe(HALF_QUOTA_AMOUNT_WEI);
    });
  });

  describe("metrics tracking", () => {
    it("records vault address when quota exceeded", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const OVER_QUOTA_AMOUNT_WEI = quotaWei + 1000000000000000000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, OVER_QUOTA_AMOUNT_WEI);

      // Assert
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).toHaveBeenCalledWith(VAULT_ADDRESS);
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).toHaveBeenCalledTimes(1);
    });

    it("skips quota exceeded metric when within limits", () => {
      // Arrange
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      const HALF_QUOTA_AMOUNT_WEI = quotaWei / 2n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, HALF_QUOTA_AMOUNT_WEI);

      // Assert
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).not.toHaveBeenCalled();
    });

    it("skips quota exceeded metric when below tolerance threshold", () => {
      // Arrange
      const BELOW_THRESHOLD_AMOUNT_WEI = 500000000000000000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, BELOW_THRESHOLD_AMOUNT_WEI);

      // Assert
      expect(metricsUpdater.incrementStakingDepositQuotaExceeded).not.toHaveBeenCalled();
    });
  });

  describe("logging verification", () => {
    it("logs service initialization", () => {
      // Arrange & Act
      service = createService();

      // Assert
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("RebalanceQuotaService initialized"));
    });

    it("logs tolerance threshold rejection", () => {
      // Arrange
      const BELOW_THRESHOLD_AMOUNT_WEI = 500000000000000000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, BELOW_THRESHOLD_AMOUNT_WEI);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("below tolerance threshold"));
    });

    it("logs quota calculation state variables", () => {
      // Arrange
      const REBALANCE_AMOUNT_WEI = 50000000000000000000n;
      const quotaWei = (TOTAL_SYSTEM_BALANCE_WEI * BigInt(REBALANCE_QUOTA_BPS)) / 10000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, REBALANCE_AMOUNT_WEI);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`totalSystemBalance=${TOTAL_SYSTEM_BALANCE_WEI.toString()}`),
      );
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`reBalanceAmountWei=${REBALANCE_AMOUNT_WEI.toString()}`),
      );
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining(`quotaWei=${quotaWei.toString()}`));
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("prevTotal"));
    });

    it("logs new total during quota processing", () => {
      // Arrange
      const REBALANCE_AMOUNT_WEI = 50000000000000000000n;
      service = createService();

      // Act
      service.getRebalanceAmountAfterQuota(VAULT_ADDRESS, TOTAL_SYSTEM_BALANCE_WEI, REBALANCE_AMOUNT_WEI);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(expect.stringContaining("newTotal"));
    });
  });
});

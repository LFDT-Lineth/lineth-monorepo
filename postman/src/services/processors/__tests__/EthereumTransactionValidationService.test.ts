import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { TestLogger } from "../../../../src/utils/testing/helpers";
import { ILineaRollupClient } from "../../../core/clients/blockchain/ethereum/ILineaRollupClient";
import { IEthereumGasProvider } from "../../../core/clients/blockchain/IGasProvider";
import {
  DEFAULT_ENABLE_POSTMAN_SPONSORING,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_PROFIT_MARGIN,
} from "../../../core/constants";
import { DEFAULT_MAX_FEE_PER_GAS, testMessage } from "../../../utils/testing/constants";
import { EthereumTransactionValidationService } from "../../EthereumTransactionValidationService";

describe("EthereumTransactionValidationService", () => {
  let lineaTransactionValidationService: EthereumTransactionValidationService;
  let lineaRollupClient: ILineaRollupClient;
  let gasProvider: IEthereumGasProvider;

  const logger = new TestLogger(EthereumTransactionValidationService.name);

  beforeEach(() => {
    lineaRollupClient = mock<ILineaRollupClient>();
    gasProvider = mock<IEthereumGasProvider>();

    lineaTransactionValidationService = new EthereumTransactionValidationService(
      lineaRollupClient,
      gasProvider,
      {
        profitMargin: DEFAULT_PROFIT_MARGIN,
        maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
        isPostmanSponsorshipEnabled: DEFAULT_ENABLE_POSTMAN_SPONSORING,
        maxPostmanSponsorGasLimit: DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
      },
      logger,
    );

    jest.spyOn(gasProvider, "getGasFees").mockResolvedValueOnce({
      maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
    });
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("evaluateTransaction", () => {
    it("Should return transaction evaluation criteria with hasZeroFee = true", async () => {
      const estimatedGasLimit = 50_000n;
      jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValueOnce(estimatedGasLimit);
      jest.spyOn(lineaRollupClient, "isRateLimitExceeded").mockResolvedValueOnce(false);

      testMessage.fee = 0n;
      const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

      expect(criteria).toStrictEqual({
        estimatedGasLimit: estimatedGasLimit,
        hasZeroFee: true,
        isRateLimitExceeded: false,
        isForSponsorship: false,
        isUnderPriced: true,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        threshold: 0,
      });
    });

    it("Should return transaction evaluation criteria with isUnderPriced = true", async () => {
      const estimatedGasLimit = 50_000n;
      jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValueOnce(estimatedGasLimit);
      jest.spyOn(lineaRollupClient, "isRateLimitExceeded").mockResolvedValueOnce(false);

      testMessage.fee = 1n;
      const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

      expect(criteria).toStrictEqual({
        estimatedGasLimit: estimatedGasLimit,
        hasZeroFee: false,
        isRateLimitExceeded: false,
        isUnderPriced: true,
        isForSponsorship: false,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        threshold: 0,
      });
    });

    it("Should return transaction evaluation criteria with estimatedGasLimit = null", async () => {
      const estimatedGasLimit = DEFAULT_MAX_CLAIM_GAS_LIMIT + 1n;
      jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValueOnce(estimatedGasLimit);
      jest.spyOn(lineaRollupClient, "isRateLimitExceeded").mockResolvedValueOnce(false);

      const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

      expect(criteria).toStrictEqual({
        estimatedGasLimit: null,
        hasZeroFee: false,
        isRateLimitExceeded: false,
        isUnderPriced: true,
        isForSponsorship: false,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        threshold: 0,
      });
    });

    it("Should return transaction evaluation criteria with isRateLimitExceeded = true", async () => {
      const estimatedGasLimit = DEFAULT_MAX_CLAIM_GAS_LIMIT + 1n;
      jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValueOnce(estimatedGasLimit);
      jest.spyOn(lineaRollupClient, "isRateLimitExceeded").mockResolvedValueOnce(true);

      const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

      expect(criteria).toStrictEqual({
        estimatedGasLimit: null,
        hasZeroFee: false,
        isRateLimitExceeded: true,
        isUnderPriced: true,
        isForSponsorship: false,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        threshold: 0,
      });
    });

    it("Should return transaction evaluation criteria for a valid message", async () => {
      const estimatedGasLimit = 50_000n;
      jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValueOnce(estimatedGasLimit);
      jest.spyOn(lineaRollupClient, "isRateLimitExceeded").mockResolvedValueOnce(false);

      testMessage.fee = 100000000000000000000n;
      const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

      expect(criteria).toStrictEqual({
        estimatedGasLimit: estimatedGasLimit,
        hasZeroFee: false,
        isRateLimitExceeded: false,
        isUnderPriced: false,
        isForSponsorship: false,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        threshold: 2000000000000000,
      });
    });

    it("When isPostmanSponsorshipEnabled is false, should return transaction evaluation criteria with isForSponsorship = false", async () => {
      const estimatedGasLimit = 50_000n;
      jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValueOnce(estimatedGasLimit);
      jest.spyOn(lineaRollupClient, "isRateLimitExceeded").mockResolvedValueOnce(false);
      testMessage.fee = 0n;

      const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

      expect(criteria.isForSponsorship).toBe(false);
    });

    describe("isPostmanSponsorshipEnabled is true", () => {
      beforeEach(() => {
        jest.spyOn(gasProvider, "getGasFees").mockResolvedValueOnce({
          maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
          maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        });
        lineaTransactionValidationService = new EthereumTransactionValidationService(
          lineaRollupClient,
          gasProvider,
          {
            profitMargin: DEFAULT_PROFIT_MARGIN,
            maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
            isPostmanSponsorshipEnabled: true,
            maxPostmanSponsorGasLimit: DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
          },
          logger,
        );
      });

      it("When gas limit < sponsor threshold, should return transaction evaluation criteria with isForSponsorship = true", async () => {
        const estimatedGasLimit = DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT - 1n;
        jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValueOnce(estimatedGasLimit);
        jest.spyOn(lineaRollupClient, "isRateLimitExceeded").mockResolvedValueOnce(false);
        testMessage.fee = 0n;

        const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

        expect(criteria.isForSponsorship).toBe(true);
      });

      it("When gas limit > sponsor threshold, should return transaction evaluation criteria with isForSponsorship = false", async () => {
        const estimatedGasLimit = DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT + 1n;
        jest.spyOn(lineaRollupClient, "estimateClaimGas").mockResolvedValueOnce(estimatedGasLimit);
        jest.spyOn(lineaRollupClient, "isRateLimitExceeded").mockResolvedValueOnce(false);

        testMessage.fee = 0n;
        const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

        expect(criteria.isForSponsorship).toBe(false);
      });
    });
  });
});

import { describe, it, beforeEach } from "@jest/globals";
import { mock, MockProxy } from "jest-mock-extended";

import { TestLogger } from "../../../../src/utils/testing/helpers";
import { IL2MessageServiceClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceClient";
import { ILineaProvider } from "../../../core/clients/blockchain/linea/ILineaProvider";
import {
  DEFAULT_ENABLE_POSTMAN_SPONSORING,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_PROFIT_MARGIN,
} from "../../../core/constants";
import { DEFAULT_MAX_FEE_PER_GAS, testMessage } from "../../../utils/testing/constants";
import { generateMessage } from "../../../utils/testing/helpers";
import { LineaTransactionValidationService } from "../../LineaTransactionValidationService";

describe("LineaTransactionValidationService", () => {
  let lineaTransactionValidationService: LineaTransactionValidationService;
  let l2ContractClient: MockProxy<IL2MessageServiceClient>;
  let provider: MockProxy<ILineaProvider>;

  const setup = (estimatedGasLimit: bigint, isNullExtraData = false) => {
    jest.spyOn(l2ContractClient, "estimateClaimGasFees").mockResolvedValueOnce({
      gasLimit: estimatedGasLimit,
      maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
    });
    jest.spyOn(l2ContractClient, "isRateLimitExceeded").mockResolvedValueOnce(false);
    jest.spyOn(provider, "getBlockExtraData").mockResolvedValueOnce(
      isNullExtraData
        ? null
        : {
            version: 1,
            variableCost: 1_000_000,
            fixedCost: 1_000_000,
            ethGasPrice: 1_000_000,
          },
    );
  };

  const logger = new TestLogger(LineaTransactionValidationService.name);

  beforeEach(() => {
    provider = mock<ILineaProvider>();
    l2ContractClient = mock<IL2MessageServiceClient>();

    lineaTransactionValidationService = new LineaTransactionValidationService(
      {
        profitMargin: DEFAULT_PROFIT_MARGIN,
        maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
        isPostmanSponsorshipEnabled: DEFAULT_ENABLE_POSTMAN_SPONSORING,
        maxPostmanSponsorGasLimit: DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
      },
      provider,
      l2ContractClient,
      logger,
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("evaluateTransaction", () => {
    it("Should throw an error when there is no extraData in the L2 block", async () => {
      const estimatedGasLimit = 50_000n;
      setup(estimatedGasLimit, true);

      await expect(lineaTransactionValidationService.evaluateTransaction(testMessage)).rejects.toThrow("No extra data");
    });

    it("Should return transaction evaluation criteria with hasZeroFee = true", async () => {
      const estimatedGasLimit = 50_000n;
      setup(estimatedGasLimit);
      testMessage.fee = 0n;

      const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

      expect(criteria).toStrictEqual({
        estimatedGasLimit: estimatedGasLimit,
        hasZeroFee: true,
        isRateLimitExceeded: false,
        isUnderPriced: true,
        isForSponsorship: false,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        threshold: 0,
      });
    });

    it("Should return transaction evaluation criteria with isUnderPriced = true", async () => {
      const estimatedGasLimit = 50_000n;
      setup(estimatedGasLimit);
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
      setup(estimatedGasLimit);

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

    it("Should return transaction evaluation criteria for a valid message", async () => {
      const estimatedGasLimit = 50_000n;
      setup(estimatedGasLimit);
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
      setup(estimatedGasLimit);
      testMessage.fee = 0n;

      const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

      expect(criteria.isForSponsorship).toBe(false);
    });

    it("Should throw BaseError when compressedTransactionSize is undefined", async () => {
      const estimatedGasLimit = 50_000n;
      setup(estimatedGasLimit);
      const messageWithoutSize = generateMessage({
        compressedTransactionSize: undefined,
        fee: 100000000000n,
      });

      await expect(lineaTransactionValidationService.evaluateTransaction(messageWithoutSize)).rejects.toThrow(
        `compressedTransactionSize is undefined for message. messageHash=${messageWithoutSize.messageHash}`,
      );
    });

    describe("isPostmanSponsorshipEnabled is true", () => {
      beforeEach(() => {
        lineaTransactionValidationService = new LineaTransactionValidationService(
          {
            profitMargin: DEFAULT_PROFIT_MARGIN,
            maxClaimGasLimit: DEFAULT_MAX_CLAIM_GAS_LIMIT,
            isPostmanSponsorshipEnabled: true,
            maxPostmanSponsorGasLimit: DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
          },
          provider,
          l2ContractClient,
          logger,
        );
      });

      it("When gas limit < sponsor threshold, should return transaction evaluation criteria with isForSponsorship = true", async () => {
        const estimatedGasLimit = DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT - 1n;
        setup(estimatedGasLimit);
        testMessage.fee = 0n;

        const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

        expect(criteria.isForSponsorship).toBe(true);
      });

      it("When gas limit > sponsor threshold, should return transaction evaluation criteria with isForSponsorship = false", async () => {
        const estimatedGasLimit = DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT + 1n;
        setup(estimatedGasLimit);
        testMessage.fee = 0n;

        const criteria = await lineaTransactionValidationService.evaluateTransaction(testMessage);

        expect(criteria.isForSponsorship).toBe(false);
      });
    });
  });
});

import { DefaultGasProvider, LineaProvider, Provider, testingHelpers } from "@consensys/linea-sdk";
import { describe, it, beforeEach } from "@jest/globals";
import {
  ContractTransactionResponse,
  ErrorDescription,
  Overrides,
  TransactionReceipt,
  TransactionResponse,
  Wallet,
} from "ethers";
import { mock } from "jest-mock-extended";

import { TestLogger } from "../../../../src/utils/testing/helpers";
import { ILineaRollupClient } from "../../../core/clients/blockchain/ethereum/ILineaRollupClient";
import {
  DEFAULT_ENABLE_POSTMAN_SPONSORING,
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_PROFIT_MARGIN,
} from "../../../core/constants";
import {
  DEFAULT_MAX_FEE_PER_GAS,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_L1_SIGNER_PRIVATE_KEY,
  testMessage,
} from "../../../utils/testing/constants";
import { EthereumTransactionValidationService } from "../../EthereumTransactionValidationService";

describe("EthereumTransactionValidationService", () => {
  let lineaTransactionValidationService: EthereumTransactionValidationService;
  let gasProvider: DefaultGasProvider;

  let lineaRollupClient: ILineaRollupClient<
    Overrides,
    TransactionReceipt,
    TransactionResponse,
    ContractTransactionResponse,
    ErrorDescription
  >;

  const logger = new TestLogger(EthereumTransactionValidationService.name);

  beforeEach(() => {
    const clients = testingHelpers.generateLineaRollupClient(
      mock<Provider>(),
      mock<LineaProvider>(),
      TEST_CONTRACT_ADDRESS_1,
      TEST_CONTRACT_ADDRESS_2,
      "read-write",
      new Wallet(TEST_L1_SIGNER_PRIVATE_KEY),
      {
        gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
        maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
        enforceMaxGasFee: false,
      },
    );
    lineaRollupClient = clients.lineaRollupClient;
    gasProvider = clients.gasProvider;

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

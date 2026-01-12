import { describe, it, beforeEach } from "@jest/globals";
import { mock, MockProxy } from "jest-mock-extended";
import {
  ContractTransactionResponse,
  ErrorDescription,
  Overrides,
  Signer,
  TransactionReceipt,
  TransactionResponse,
  Wallet,
} from "ethers";
import { GasProvider, LineaProvider, testingHelpers } from "@consensys/linea-sdk";
import {
  DEFAULT_MAX_FEE_PER_GAS,
  TEST_CONTRACT_ADDRESS_2,
  TEST_L2_SIGNER_PRIVATE_KEY,
  testMessage,
} from "../../../utils/testing/constants";
import {
  DEFAULT_ENABLE_POSTMAN_SPONSORING,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_PROFIT_MARGIN,
} from "../../../core/constants";
import { IL2MessageServiceClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceClient";
import { LineaTransactionValidationService } from "../../LineaTransactionValidationService";
import { TestLogger } from "../../../../src/utils/testing/helpers";

describe("LineaTransactionValidationService", () => {
  let lineaTransactionValidationService: LineaTransactionValidationService;
  let gasProvider: GasProvider;
  let l2ContractClient: IL2MessageServiceClient<
    Overrides,
    TransactionReceipt,
    TransactionResponse,
    ContractTransactionResponse,
    Signer,
    ErrorDescription
  >;
  let provider: MockProxy<LineaProvider>;

  const setup = (estimatedGasLimit: bigint, isNullExtraData = false) => {
    jest.spyOn(gasProvider, "getGasFees").mockResolvedValueOnce({
      gasLimit: estimatedGasLimit,
      maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
    });
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
    provider = mock<LineaProvider>();
    const clients = testingHelpers.generateL2MessageServiceClient(
      provider,
      TEST_CONTRACT_ADDRESS_2,
      "read-write",
      new Wallet(TEST_L2_SIGNER_PRIVATE_KEY),
      {
        maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
        enforceMaxGasFee: false,
      },
    );

    l2ContractClient = clients.l2MessageServiceClient;
    gasProvider = clients.gasProvider;

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

    jest.spyOn(l2ContractClient, "getSigner").mockReturnValueOnce(new Wallet(TEST_L2_SIGNER_PRIVATE_KEY));
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

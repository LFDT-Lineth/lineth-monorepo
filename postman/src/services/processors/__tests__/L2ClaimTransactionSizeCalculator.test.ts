import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import {
  ContractTransactionResponse,
  ErrorDescription,
  Overrides,
  Signer,
  TransactionReceipt,
  TransactionResponse,
  Wallet,
} from "ethers";
import { LineaProvider, serialize, testingHelpers } from "@consensys/linea-sdk";
import {
  DEFAULT_MAX_FEE_PER_GAS,
  TEST_CONTRACT_ADDRESS_2,
  TEST_L2_SIGNER_PRIVATE_KEY,
  testMessage,
} from "../../../utils/testing/constants";
import { EthereumMessageDBService } from "../../persistence/EthereumMessageDBService";
import { L2ClaimTransactionSizeCalculator } from "../../L2ClaimTransactionSizeCalculator";
import { DEFAULT_MAX_FEE_PER_GAS_CAP } from "../../../core/constants";
import { BaseError } from "../../../core/errors";
import { IL2MessageServiceClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceClient";

describe("L2ClaimTransactionSizeCalculator", () => {
  let transactionSizeCalculator: L2ClaimTransactionSizeCalculator;

  const databaseService = mock<EthereumMessageDBService>();
  let l2ContractClient: IL2MessageServiceClient<
    Overrides,
    TransactionReceipt,
    TransactionResponse,
    ContractTransactionResponse,
    Signer,
    ErrorDescription
  >;

  beforeEach(() => {
    const clients = testingHelpers.generateL2MessageServiceClient(
      mock<LineaProvider>(),
      TEST_CONTRACT_ADDRESS_2,
      "read-only",
      undefined,
      {
        maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
        enforceMaxGasFee: false,
      },
    );

    l2ContractClient = clients.l2MessageServiceClient;
    transactionSizeCalculator = new L2ClaimTransactionSizeCalculator(l2ContractClient);
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("process", () => {
    it("Should throw an error if  signer is undefined", async () => {
      jest.spyOn(databaseService, "getNFirstMessagesByStatus").mockResolvedValue([]);

      await expect(
        transactionSizeCalculator.calculateTransactionSize(testMessage, {
          gasLimit: 50_000n,
          maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
          maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        }),
      ).rejects.toThrow(
        new BaseError(`Transaction size calculation error: ${serialize(new BaseError("Signer is undefined."))}`),
      );
    });

    it("Should return transaction size", async () => {
      jest.spyOn(l2ContractClient, "getSigner").mockReturnValueOnce(new Wallet(TEST_L2_SIGNER_PRIVATE_KEY));

      const transactionSize = await transactionSizeCalculator.calculateTransactionSize(testMessage, {
        gasLimit: 50_000n,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });

      expect(transactionSize).toStrictEqual(77);
    });
  });
});

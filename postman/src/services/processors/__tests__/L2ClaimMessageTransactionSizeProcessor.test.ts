import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import {
  ContractTransactionResponse,
  ErrorDescription,
  Overrides,
  Signer,
  TransactionReceipt,
  TransactionResponse,
} from "ethers";
import { Direction } from "@consensys/linea-sdk";
import { TestLogger } from "../../../utils/testing/helpers";
import { MessageStatus } from "../../../core/enums";
import { testL1NetworkConfig, testMessage, DEFAULT_MAX_FEE_PER_GAS } from "../../../utils/testing/constants";
import { EthereumMessageDBService } from "../../persistence/EthereumMessageDBService";
import { L2ClaimMessageTransactionSizeProcessor } from "../L2ClaimMessageTransactionSizeProcessor";
import { L2ClaimTransactionSizeCalculator } from "../../L2ClaimTransactionSizeCalculator";
import { IL2MessageServiceClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceClient";

describe("L2ClaimMessageTransactionSizeProcessor", () => {
  let transactionSizeProcessor: L2ClaimMessageTransactionSizeProcessor;
  let transactionSizeCalculator: L2ClaimTransactionSizeCalculator;

  const databaseService = mock<EthereumMessageDBService>();
  const l2ContractClientMock =
    mock<
      IL2MessageServiceClient<
        Overrides,
        TransactionReceipt,
        TransactionResponse,
        ContractTransactionResponse,
        Signer,
        ErrorDescription
      >
    >();
  const logger = new TestLogger(L2ClaimMessageTransactionSizeProcessor.name);

  beforeEach(() => {
    transactionSizeCalculator = new L2ClaimTransactionSizeCalculator(l2ContractClientMock);

    transactionSizeProcessor = new L2ClaimMessageTransactionSizeProcessor(
      databaseService,
      l2ContractClientMock,
      transactionSizeCalculator,
      {
        direction: Direction.L1_TO_L2,
        originContractAddress: testL1NetworkConfig.messageServiceContractAddress,
      },
      logger,
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("process", () => {
    it("Should return if getNFirstMessagesByStatus returns empty list", async () => {
      const transactionSizeCalculatorSpy = jest.spyOn(transactionSizeCalculator, "calculateTransactionSize");
      jest.spyOn(databaseService, "getNFirstMessagesByStatus").mockResolvedValue([]);

      await transactionSizeProcessor.process();

      expect(transactionSizeCalculatorSpy).toHaveBeenCalledTimes(0);
    });

    it("Should log as error when calculateTransactionSize failed", async () => {
      const testGasLimit = 50_000n;

      const loggerErrorSpy = jest.spyOn(logger, "error");
      jest.spyOn(databaseService, "getNFirstMessagesByStatus").mockResolvedValue([testMessage]);
      jest.spyOn(l2ContractClientMock, "estimateClaimGasFees").mockResolvedValue({
        gasLimit: testGasLimit,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });
      jest
        .spyOn(transactionSizeCalculator, "calculateTransactionSize")
        .mockRejectedValueOnce(new Error("calculation failed."));

      await transactionSizeProcessor.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith(new Error("calculation failed."));
    });

    it("Should log as info and call updateMessage if the transaction size calculation succeed", async () => {
      const testGasLimit = 50_000n;
      const testTransactionSize = 100;

      const loggerInfoSpy = jest.spyOn(logger, "info");
      jest.spyOn(databaseService, "getNFirstMessagesByStatus").mockResolvedValue([testMessage]);
      jest.spyOn(databaseService, "updateMessage").mockImplementationOnce(jest.fn());
      jest.spyOn(l2ContractClientMock, "estimateClaimGasFees").mockResolvedValue({
        gasLimit: testGasLimit,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });
      jest.spyOn(transactionSizeCalculator, "calculateTransactionSize").mockResolvedValue(testTransactionSize);

      const testMessageEditSpy = jest.spyOn(testMessage, "edit");
      const databaseServiceMockUpdateSpy = jest.spyOn(databaseService, "updateMessage");

      await transactionSizeProcessor.process();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith(
        "Message transaction size and gas limit have been computed: messageHash=%s transactionSize=%s gasLimit=%s",
        testMessage.messageHash,
        testTransactionSize,
        testGasLimit,
      );
      expect(testMessageEditSpy).toHaveBeenCalledTimes(1);
      expect(testMessageEditSpy).toHaveBeenCalledWith({
        claimTxGasLimit: Number(testGasLimit),
        compressedTransactionSize: testTransactionSize,
        status: MessageStatus.TRANSACTION_SIZE_COMPUTED,
      });
      expect(databaseServiceMockUpdateSpy).toHaveBeenCalledTimes(1);
      expect(databaseServiceMockUpdateSpy).toHaveBeenCalledWith(testMessage);
    });
  });
});

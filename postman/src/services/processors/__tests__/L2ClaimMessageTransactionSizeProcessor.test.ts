import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { IL2MessageServiceClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceClient";
import { Direction, MessageStatus } from "../../../core/enums";
import { IMessageRepository } from "../../../core/persistence/IMessageRepository";
import { ITransactionSigner } from "../../../core/services/ITransactionSigner";
import { L2ClaimTransactionSizeCalculator } from "../../../infrastructure/blockchain/L2ClaimTransactionSizeCalculator";
import { testL1NetworkConfig, testMessage, DEFAULT_MAX_FEE_PER_GAS } from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { L2ClaimMessageTransactionSizeProcessor } from "../L2ClaimMessageTransactionSizeProcessor";

describe("L2ClaimMessageTransactionSizeProcessor", () => {
  let transactionSizeProcessor: L2ClaimMessageTransactionSizeProcessor;
  let transactionSizeCalculator: L2ClaimTransactionSizeCalculator;

  const databaseService = mock<IMessageRepository>();
  const l2ContractClientMock = mock<IL2MessageServiceClient>();
  const transactionSignerMock = mock<ITransactionSigner>();
  const logger = new TestLogger(L2ClaimMessageTransactionSizeProcessor.name);
  const errorParser = { parse: jest.fn() };

  beforeEach(() => {
    errorParser.parse.mockReturnValue({ retryable: false, message: "" });
    transactionSizeCalculator = new L2ClaimTransactionSizeCalculator(l2ContractClientMock, transactionSignerMock);

    transactionSizeProcessor = new L2ClaimMessageTransactionSizeProcessor(
      databaseService,
      l2ContractClientMock,
      transactionSizeCalculator,
      {
        direction: Direction.L1_TO_L2,
        originContractAddress: testL1NetworkConfig.messageServiceContractAddress,
      },
      logger,
      errorParser,
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
      expect(loggerErrorSpy).toHaveBeenCalledWith("Non-retryable error computing transaction size.", {
        error: new Error("calculation failed."),
        parsedError: { retryable: false, message: "" },
        messageHash: testMessage.messageHash,
      });
    });

    it("Should log as error when estimateClaimGasFees failed", async () => {
      const loggerErrorSpy = jest.spyOn(logger, "error");
      jest.spyOn(databaseService, "getNFirstMessagesByStatus").mockResolvedValue([testMessage]);
      const error = new Error("could not coalesce error");
      jest.spyOn(l2ContractClientMock, "estimateClaimGasFees").mockRejectedValue(error);

      await transactionSizeProcessor.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Non-retryable error computing transaction size.", {
        error,
        parsedError: { retryable: false, message: "" },
        messageHash: testMessage.messageHash,
      });
    });

    it("Should log generic error when error is retryable", async () => {
      errorParser.parse.mockReturnValue({ retryable: true, message: "rate limited" });
      const loggerErrorSpy = jest.spyOn(logger, "error");
      const error = new Error("rate limit hit");
      jest.spyOn(databaseService, "getNFirstMessagesByStatus").mockResolvedValue([testMessage]);
      jest.spyOn(l2ContractClientMock, "estimateClaimGasFees").mockRejectedValue(error);

      await transactionSizeProcessor.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error computing transaction size.", {
        error,
        parsedError: { retryable: true, message: "rate limited" },
      });
    });

    it("Should log generic error when message is null (error before message assignment)", async () => {
      errorParser.parse.mockReturnValue({ retryable: false, message: "" });
      const loggerErrorSpy = jest.spyOn(logger, "error");
      const error = new Error("db failure");
      jest.spyOn(databaseService, "getNFirstMessagesByStatus").mockRejectedValue(error);

      await transactionSizeProcessor.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error computing transaction size.", {
        error,
        parsedError: { retryable: false, message: "" },
      });
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
      expect(loggerInfoSpy).toHaveBeenCalledWith("Message transaction size and gas limit have been computed.", {
        messageHash: testMessage.messageHash,
        transactionSize: testTransactionSize,
        gasLimit: testGasLimit.toString(),
      });
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

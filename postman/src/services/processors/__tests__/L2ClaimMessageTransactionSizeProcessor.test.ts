import { Direction, makeBaseError } from "@consensys/linea-sdk";
import { describe, it, beforeEach } from "@jest/globals";
import {
  ContractTransactionResponse,
  ErrorDescription,
  makeError,
  Overrides,
  Signer,
  TransactionReceipt,
  TransactionResponse,
} from "ethers";
import { mock } from "jest-mock-extended";

import { IL2MessageServiceClient } from "../../../core/clients/blockchain/linea/IL2MessageServiceClient";
import { MessageStatus } from "../../../core/enums";
import { testL1NetworkConfig, testMessage, DEFAULT_MAX_FEE_PER_GAS } from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { L2ClaimTransactionSizeCalculator } from "../../L2ClaimTransactionSizeCalculator";
import { EthereumMessageDBService } from "../../persistence/EthereumMessageDBService";
import { L2ClaimMessageTransactionSizeProcessor } from "../L2ClaimMessageTransactionSizeProcessor";

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

      const loggerErrorSpy = jest.spyOn(logger, "warnOrError");
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
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error occurred while processing message transaction size.", {
        error: new Error("calculation failed."),
        parsedError: {
          errorCode: "UNKNOWN_ERROR",
          errorMessage: "calculation failed.",
          mitigation: { shouldRetry: false },
        },
        messageHash: testMessage.messageHash,
      });
    });

    it("Should log as error when estimateClaimGasFees failed", async () => {
      const loggerErrorSpy = jest.spyOn(logger, "warnOrError");
      jest.spyOn(databaseService, "getNFirstMessagesByStatus").mockResolvedValue([testMessage]);
      const error = makeError("could not coalesce error", "UNKNOWN_ERROR", {
        error: {
          code: -32000,
          data: "0x5461344300000000000000000000000034be5b8c30ee4fde069dc878989686abe9884470",
          message: "Execution reverted",
        },
        payload: {
          id: 1,
          jsonrpc: "2.0",
          method: "linea_estimateGas",
          params: [
            {
              data: "0x491e09360000000000000000000000004420ce157f2c39edaae6cc107a42c8e527d6e02800000000000000000000000034be5b8c30ee4fde069dc878989686abe988447000000000000000000000000000000000000000000000000000006182ba2f0b400000000000000000000000000000000000000000000000000001c6bf52634000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000052b130000000000000000000000000000000000000000000000000000000000000000",
              from: "0x46eA7a855DA88FBC09cc59de93468E6bFbf0d81b",
              to: "0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec",
              value: "0",
            },
          ],
        },
      });
      jest.spyOn(l2ContractClientMock, "estimateClaimGasFees").mockRejectedValue(makeBaseError(error, testMessage));

      await transactionSizeProcessor.process();

      expect(loggerErrorSpy).toHaveBeenCalledTimes(1);
      expect(loggerErrorSpy).toHaveBeenCalledWith("Error occurred while processing message transaction size.", {
        parsedError: {
          data: "0x5461344300000000000000000000000034be5b8c30ee4fde069dc878989686abe9884470",
          errorCode: "UNKNOWN_ERROR",
          errorMessage: "Execution reverted",

          mitigation: { shouldRetry: false },
        },
        error,
        messageHash: testMessage.messageHash,
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

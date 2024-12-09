import { describe, it, beforeEach } from "@jest/globals";
import { MockProxy, mock } from "jest-mock-extended";
import { Direction } from "@consensys/linea-sdk";
import { TestLogger } from "../../../utils/testing/helpers";
import { testL2NetworkConfig } from "../../../utils/testing/constants";
import { IPoller } from "../../../core/services/pollers/IPoller";
import { L2ClaimMessageTransactionSizePoller } from "../L2ClaimMessageTransactionSizePoller";
import { L2ClaimMessageTransactionSizeProcessor } from "../../processors/L2ClaimMessageTransactionSizeProcessor";

describe("L2ClaimMessageTransactionSizePoller", () => {
  let testClaimMessageTransactionSizePoller: IPoller;
  let transactionSizeProcessor: MockProxy<L2ClaimMessageTransactionSizeProcessor>;
  const logger = new TestLogger(L2ClaimMessageTransactionSizePoller.name);

  beforeEach(() => {
    transactionSizeProcessor = mock<L2ClaimMessageTransactionSizeProcessor>();

    testClaimMessageTransactionSizePoller = new L2ClaimMessageTransactionSizePoller(
      transactionSizeProcessor,
      {
        pollingInterval: testL2NetworkConfig.listener.pollingInterval,
      },
      logger,
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("start", () => {
    it("Should return and log as warning if it has been started", async () => {
      const loggerWarnSpy = jest.spyOn(logger, "warn");

      testClaimMessageTransactionSizePoller.start();
      await testClaimMessageTransactionSizePoller.start();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("%s has already started.", L2ClaimMessageTransactionSizePoller.name);
    });

    it("Should call process and log as info if it started successfully", async () => {
      const transactionSizeProcessorMockSpy = jest
        .spyOn(transactionSizeProcessor, "process")
        .mockImplementation(jest.fn());
      const loggerInfoSpy = jest.spyOn(logger, "info");

      testClaimMessageTransactionSizePoller.start();

      expect(transactionSizeProcessorMockSpy).toHaveBeenCalled();
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith(
        "Starting %s %s...",
        Direction.L1_TO_L2,
        L2ClaimMessageTransactionSizePoller.name,
      );
      testClaimMessageTransactionSizePoller.stop();
    });
  });

  describe("stop", () => {
    it("Should return and log as info if it stopped successfully", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      testClaimMessageTransactionSizePoller = new L2ClaimMessageTransactionSizePoller(
        transactionSizeProcessor,
        {
          pollingInterval: testL2NetworkConfig.listener.pollingInterval,
        },
        logger,
      );

      testClaimMessageTransactionSizePoller.stop();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(2);
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        1,
        "Stopping %s %s...",
        Direction.L1_TO_L2,
        L2ClaimMessageTransactionSizePoller.name,
      );
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        2,
        "%s %s stopped.",
        Direction.L1_TO_L2,
        L2ClaimMessageTransactionSizePoller.name,
      );
    });
  });
});

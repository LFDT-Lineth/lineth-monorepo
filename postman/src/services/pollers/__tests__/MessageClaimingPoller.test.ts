import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { Direction } from "@consensys/linea-sdk";
import { MessageClaimingPoller } from "../MessageClaimingPoller";
import { TestLogger } from "../../../utils/testing/helpers";
import { IMessageClaimingProcessor } from "../../../core/services/processors/IMessageClaimingProcessor";
import { testL2NetworkConfig } from "../../../utils/testing/constants";
import { IPoller } from "../../../core/services/pollers/IPoller";

describe("TestMessageClaimingPoller", () => {
  let testClaimingPoller: IPoller;
  const claimingProcessorMock = mock<IMessageClaimingProcessor>();
  const logger = new TestLogger(MessageClaimingPoller.name);

  beforeEach(() => {
    testClaimingPoller = new MessageClaimingPoller(
      claimingProcessorMock,
      {
        direction: Direction.L1_TO_L2,
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

      testClaimingPoller.start();
      await testClaimingPoller.start();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("%s has already started.", MessageClaimingPoller.name);
    });

    it("Should call getAndClaimAnchoredMessage and log as info if it started successfully", async () => {
      const claimingProcessorMockSpy = jest.spyOn(claimingProcessorMock, "process");
      const loggerInfoSpy = jest.spyOn(logger, "info");

      testClaimingPoller.start();

      expect(claimingProcessorMockSpy).toHaveBeenCalled();
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Starting %s %s...", Direction.L1_TO_L2, MessageClaimingPoller.name);
    });
  });

  describe("stop", () => {
    it("Should return and log as info if it stopped successfully", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      testClaimingPoller = new MessageClaimingPoller(
        claimingProcessorMock,
        {
          direction: Direction.L1_TO_L2,
          pollingInterval: testL2NetworkConfig.listener.pollingInterval,
        },
        logger,
      );

      testClaimingPoller.stop();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(2);
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        1,
        "Stopping %s %s...",
        Direction.L1_TO_L2,
        MessageClaimingPoller.name,
      );
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        2,
        "%s %s stopped.",
        Direction.L1_TO_L2,
        MessageClaimingPoller.name,
      );
    });
  });
});

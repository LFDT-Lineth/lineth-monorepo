import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { Direction } from "@consensys/linea-sdk";
import { MessageAnchoringPoller } from "../MessageAnchoringPoller";
import { TestLogger } from "../../../utils/testing/helpers";
import { IMessageAnchoringProcessor } from "../../../core/services/processors/IMessageAnchoringProcessor";
import { testL2NetworkConfig } from "../../../utils/testing/constants";
import { IPoller } from "../../../core/services/pollers/IPoller";

describe("TestMessageAnchoringPoller", () => {
  let testAnchoringPoller: IPoller;
  const anchoringProcessorMock = mock<IMessageAnchoringProcessor>();
  const logger = new TestLogger(MessageAnchoringPoller.name);

  beforeEach(() => {
    testAnchoringPoller = new MessageAnchoringPoller(
      anchoringProcessorMock,
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

      testAnchoringPoller.start();
      await testAnchoringPoller.start();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("%s has already started.", MessageAnchoringPoller.name);
    });

    it("Should call getAndUpdateAnchoredMessageStatus and log as info if it started successfully", async () => {
      const anchoringProcessorMockSpy = jest.spyOn(anchoringProcessorMock, "process");
      const loggerInfoSpy = jest.spyOn(logger, "info");

      testAnchoringPoller.start();

      expect(anchoringProcessorMockSpy).toHaveBeenCalled();
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Starting %s %s...", Direction.L1_TO_L2, MessageAnchoringPoller.name);
    });
  });

  describe("stop", () => {
    it("Should return and log as info if it stopped successfully", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      testAnchoringPoller = new MessageAnchoringPoller(
        anchoringProcessorMock,
        {
          direction: Direction.L1_TO_L2,
          pollingInterval: testL2NetworkConfig.listener.pollingInterval,
        },
        logger,
      );

      testAnchoringPoller.stop();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(2);
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        1,
        "Stopping %s %s...",
        Direction.L1_TO_L2,
        MessageAnchoringPoller.name,
      );
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        2,
        "%s %s stopped.",
        Direction.L1_TO_L2,
        MessageAnchoringPoller.name,
      );
    });
  });
});

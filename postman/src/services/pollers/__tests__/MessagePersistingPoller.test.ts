import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { Direction } from "@consensys/linea-sdk";
import { MessagePersistingPoller } from "../MessagePersistingPoller";
import { TestLogger } from "../../../utils/testing/helpers";
import { IMessageClaimingPersister } from "../../../core/services/processors/IMessageClaimingPersister";
import { testL2NetworkConfig } from "../../../utils/testing/constants";
import { IPoller } from "../../../core/services/pollers/IPoller";

describe("TestMessagePersistingPoller", () => {
  let testPersistingPoller: IPoller;
  const claimingPersisterMock = mock<IMessageClaimingPersister>();
  const logger = new TestLogger(MessagePersistingPoller.name);

  beforeEach(() => {
    testPersistingPoller = new MessagePersistingPoller(
      claimingPersisterMock,
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

      testPersistingPoller.start();
      await testPersistingPoller.start();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("%s has already started.", MessagePersistingPoller.name);
    });

    it("Should call updateAndPersistPendingMessage and log as info if it started successfully", async () => {
      const claimingPersisterMockSpy = jest.spyOn(claimingPersisterMock, "process");
      const loggerInfoSpy = jest.spyOn(logger, "info");

      testPersistingPoller.start();

      expect(claimingPersisterMockSpy).toHaveBeenCalled();
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Starting %s %s...", Direction.L1_TO_L2, MessagePersistingPoller.name);
    });
  });

  describe("stop", () => {
    it("Should return and log as info if it stopped successfully", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");
      testPersistingPoller = new MessagePersistingPoller(
        claimingPersisterMock,
        {
          direction: Direction.L1_TO_L2,
          pollingInterval: testL2NetworkConfig.listener.pollingInterval,
        },
        logger,
      );

      testPersistingPoller.stop();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(2);
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        1,
        "Stopping %s %s...",
        Direction.L1_TO_L2,
        MessagePersistingPoller.name,
      );
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(
        2,
        "%s %s stopped.",
        Direction.L1_TO_L2,
        MessagePersistingPoller.name,
      );
    });
  });
});

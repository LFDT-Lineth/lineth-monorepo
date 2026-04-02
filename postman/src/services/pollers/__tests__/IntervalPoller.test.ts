import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { Direction } from "../../../core/enums";
import { IPoller } from "../../../core/services/pollers/IPoller";
import { wait } from "../../../core/utils/shared";
import { testL2NetworkConfig } from "../../../utils/testing/constants";
import { TestLogger } from "../../../utils/testing/helpers";
import { IntervalPoller, IProcessable } from "../IntervalPoller";

describe("IntervalPoller", () => {
  let poller: IPoller;
  const processorMock = mock<IProcessable>();
  const loggerName = "TestIntervalPoller";
  const logger = new TestLogger(loggerName);

  beforeEach(() => {
    poller = new IntervalPoller(
      processorMock,
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

      poller.start();
      await poller.start();

      expect(loggerWarnSpy).toHaveBeenCalledTimes(1);
      expect(loggerWarnSpy).toHaveBeenCalledWith("Poller has already started.", { name: loggerName });
    });

    it("Should call process and log as info if it started successfully", async () => {
      const processorSpy = jest.spyOn(processorMock, "process");
      const loggerInfoSpy = jest.spyOn(logger, "info");

      poller.start();

      expect(processorSpy).toHaveBeenCalled();
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      expect(loggerInfoSpy).toHaveBeenCalledWith("Starting poller.", {
        direction: Direction.L1_TO_L2,
        name: loggerName,
      });
    });

    it("Should omit direction from logs when not configured", async () => {
      const pollerNoDirection = new IntervalPoller(
        processorMock,
        { pollingInterval: testL2NetworkConfig.listener.pollingInterval },
        logger,
      );
      const loggerInfoSpy = jest.spyOn(logger, "info");

      pollerNoDirection.start();

      expect(loggerInfoSpy).toHaveBeenCalledWith("Starting poller.", {
        name: loggerName,
      });
    });

    it("Should log the error and continue polling when process throws", async () => {
      const fastPoller = new IntervalPoller(
        processorMock,
        { direction: Direction.L1_TO_L2, pollingInterval: 10 },
        logger,
      );
      const processError = new Error("processor blew up");
      let callCount = 0;
      jest.spyOn(processorMock, "process").mockImplementation(async () => {
        callCount++;
        if (callCount === 1) throw processError;
      });
      const loggerErrorSpy = jest.spyOn(logger, "error");

      fastPoller.start();
      await wait(50);

      expect(loggerErrorSpy).toHaveBeenCalledWith("Unhandled error in polling loop — continuing.", {
        direction: Direction.L1_TO_L2,
        name: loggerName,
        error: processError,
      });
      expect(callCount).toBeGreaterThanOrEqual(2);

      fastPoller.stop();
    });
  });

  describe("stop", () => {
    it("Should return and log as info if it stopped successfully", async () => {
      const loggerInfoSpy = jest.spyOn(logger, "info");

      poller.stop();

      expect(loggerInfoSpy).toHaveBeenCalledTimes(2);
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(1, "Stopping poller.", {
        direction: Direction.L1_TO_L2,
        name: loggerName,
      });
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(2, "Poller stopped.", {
        direction: Direction.L1_TO_L2,
        name: loggerName,
      });
    });

    it("Should omit direction from stop logs when not configured", () => {
      const pollerNoDirection = new IntervalPoller(
        processorMock,
        { pollingInterval: testL2NetworkConfig.listener.pollingInterval },
        logger,
      );
      const loggerInfoSpy = jest.spyOn(logger, "info");

      pollerNoDirection.stop();

      expect(loggerInfoSpy).toHaveBeenNthCalledWith(1, "Stopping poller.", {
        name: loggerName,
      });
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(2, "Poller stopped.", {
        name: loggerName,
      });
    });
  });

  describe("error handling without direction", () => {
    it("Should omit direction from error logs when not configured", async () => {
      const pollerNoDirection = new IntervalPoller(processorMock, { pollingInterval: 10 }, logger);
      const processError = new Error("processor blew up");
      let callCount = 0;
      jest.spyOn(processorMock, "process").mockImplementation(async () => {
        callCount++;
        if (callCount === 1) throw processError;
      });
      const loggerErrorSpy = jest.spyOn(logger, "error");

      pollerNoDirection.start();
      await wait(50);

      expect(loggerErrorSpy).toHaveBeenCalledWith("Unhandled error in polling loop — continuing.", {
        name: loggerName,
        error: processError,
      });

      pollerNoDirection.stop();
    });
  });
});

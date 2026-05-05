import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

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
      { pollingInterval: testL2NetworkConfig.listener.pollingInterval },
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
      expect(loggerWarnSpy).toHaveBeenCalledWith("Poller has already started.");
    });

    it("Should call process and log as info if it started successfully", async () => {
      const processorSpy = jest.spyOn(processorMock, "process");
      const loggerInfoSpy = jest.spyOn(logger, "info");

      poller.start();

      expect(processorSpy).toHaveBeenCalled();
      expect(loggerInfoSpy).toHaveBeenCalledTimes(1);
      // `direction` is injected upstream by the direction-bearing child logger
      // (see L1ToL2App / L2ToL1App); it is not a per-call metadata field here.
      expect(loggerInfoSpy).toHaveBeenCalledWith("Starting poller.", {
        pollingInterval: testL2NetworkConfig.listener.pollingInterval,
      });
    });

    it("Should log the error and continue polling when process throws", async () => {
      const fastPoller = new IntervalPoller(processorMock, { pollingInterval: 10 }, logger);
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
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(1, "Stopping poller.");
      expect(loggerInfoSpy).toHaveBeenNthCalledWith(2, "Poller stopped.");
    });
  });
});

import { describe, it, expect, beforeEach, afterEach } from "@jest/globals";

import { buildL2ToL1Deps } from "../../../../utils/testing/fixtures";
import { L2ToL1App } from "../L2ToL1App";

jest.mock("@consensys/linea-shared-utils", () => ({
  WinstonLogger: jest.fn().mockImplementation(() => ({
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    name: "test",
  })),
}));

describe("L2ToL1App", () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("should construct without errors", () => {
    const deps = buildL2ToL1Deps();
    expect(() => new L2ToL1App(deps)).not.toThrow();
  });

  it("should create 4 pollers (event, anchoring, claiming, persisting)", () => {
    const deps = buildL2ToL1Deps();
    const app = new L2ToL1App(deps);

    const pollers = (app as any).pollers;
    expect(pollers).toHaveLength(4);
  });

  describe("start", () => {
    let app: L2ToL1App;

    beforeEach(() => {
      app = new L2ToL1App(buildL2ToL1Deps());
    });

    it("should call start on all pollers", () => {
      const pollers = (app as any).pollers;
      const startSpies = pollers.map((p: { start: () => void }) => jest.spyOn(p, "start"));

      app.start();

      for (const spy of startSpies) {
        expect(spy).toHaveBeenCalledTimes(1);
      }
    });
  });

  describe("stop", () => {
    let app: L2ToL1App;

    beforeEach(() => {
      app = new L2ToL1App(buildL2ToL1Deps());
    });

    it("should call stop on all pollers", () => {
      const pollers = (app as any).pollers;
      const stopSpies = pollers.map((p: { stop: () => void }) => jest.spyOn(p, "stop"));

      app.stop();

      for (const spy of stopSpies) {
        expect(spy).toHaveBeenCalledTimes(1);
      }
    });
  });

  describe("getNextMessageToClaim closure", () => {
    it("should wire getNextMessageToClaim to call gasProvider and messageRepository", async () => {
      const deps = buildL2ToL1Deps();

      (deps.l1GasProvider.getGasFees as any).mockResolvedValue({ maxFeePerGas: 1000n, maxPriorityFeePerGas: 100n });

      (deps.messageRepository.getFirstMessageToClaimOnL1 as any).mockResolvedValue(null);
      const app = new L2ToL1App(deps);

      const pollers = (app as any).pollers;

      const claimingProcessor = (pollers[2] as any).processor;

      const getNext = (claimingProcessor as any).getNextMessageToClaim;
      await getNext();
      expect(deps.l1GasProvider.getGasFees).toHaveBeenCalled();
      expect(deps.messageRepository.getFirstMessageToClaimOnL1).toHaveBeenCalled();
    });
  });
});

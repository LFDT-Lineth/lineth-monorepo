import { describe, it, expect, beforeEach, afterEach } from "@jest/globals";

import { buildL1ToL2Deps } from "../../../../utils/testing/fixtures";
import { L1ToL2App } from "../L1ToL2App";

jest.mock("@consensys/linea-shared-utils", () => ({
  WinstonLogger: jest.fn().mockImplementation(() => ({
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    name: "test",
  })),
}));

describe("L1ToL2App", () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("should construct without errors", () => {
    const deps = buildL1ToL2Deps();
    expect(() => new L1ToL2App(deps)).not.toThrow();
  });

  it("should create 5 pollers (event, anchoring, claiming, persisting, size)", () => {
    const deps = buildL1ToL2Deps();
    const app = new L1ToL2App(deps);

    // Access private field via cast to verify all pollers are wired

    const pollers = (app as any).pollers;
    expect(pollers).toHaveLength(5);
  });

  describe("start", () => {
    let app: L1ToL2App;

    beforeEach(() => {
      app = new L1ToL2App(buildL1ToL2Deps());
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
    let app: L1ToL2App;

    beforeEach(() => {
      app = new L1ToL2App(buildL1ToL2Deps());
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
    it("should wire getNextMessageToClaim to call messageRepository.getFirstMessageToClaimOnL2", async () => {
      const deps = buildL1ToL2Deps();
      jest.spyOn(deps.messageRepository, "getFirstMessageToClaimOnL2").mockResolvedValue(null);
      const app = new L1ToL2App(deps);

      const pollers = (app as any).pollers;
      // claimingPoller is at index 2, wrapping the MessageClaimingProcessor

      const claimingProcessor = (pollers[2] as any).processor;

      const getNext = (claimingProcessor as any).getNextMessageToClaim;
      await getNext();
      expect(deps.messageRepository.getFirstMessageToClaimOnL2).toHaveBeenCalled();
    });
  });
});

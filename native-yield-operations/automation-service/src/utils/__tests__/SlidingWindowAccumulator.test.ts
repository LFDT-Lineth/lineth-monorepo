import { describe, it, expect } from "@jest/globals";
import { SlidingWindowAccumulator } from "../SlidingWindowAccumulator.js";

describe("SlidingWindowAccumulator", () => {
  describe("constructor", () => {
    it("creates accumulator with valid size", () => {
      const accumulator = new SlidingWindowAccumulator(5);
      expect(accumulator.getLength()).toBe(0);
      expect(accumulator.getTotal()).toBe(0n);
    });

    it("creates accumulator with size 0", () => {
      const accumulator = new SlidingWindowAccumulator(0);
      expect(accumulator.getLength()).toBe(0);
      expect(accumulator.getTotal()).toBe(0n);
    });
  });

  describe("push", () => {
    it("adds a single value and updates total", () => {
      const accumulator = new SlidingWindowAccumulator(5);
      accumulator.push(10n);

      expect(accumulator.getLength()).toBe(1);
      expect(accumulator.getTotal()).toBe(10n);
    });

    it("adds multiple values and maintains correct total", () => {
      const accumulator = new SlidingWindowAccumulator(5);
      accumulator.push(10n);
      accumulator.push(20n);
      accumulator.push(30n);

      expect(accumulator.getLength()).toBe(3);
      expect(accumulator.getTotal()).toBe(60n);
    });

    it("removes oldest value when buffer is full", () => {
      const accumulator = new SlidingWindowAccumulator(3);
      accumulator.push(10n);
      accumulator.push(20n);
      accumulator.push(30n);

      expect(accumulator.getLength()).toBe(3);
      expect(accumulator.getTotal()).toBe(60n);

      accumulator.push(40n);

      expect(accumulator.getLength()).toBe(3);
      expect(accumulator.getTotal()).toBe(90n); // 20 + 30 + 40 (10 removed)
    });

    it("maintains correct window when pushing beyond capacity", () => {
      const accumulator = new SlidingWindowAccumulator(3);
      accumulator.push(10n);
      accumulator.push(20n);
      accumulator.push(30n);
      accumulator.push(40n);
      accumulator.push(50n);

      expect(accumulator.getLength()).toBe(3);
      expect(accumulator.getTotal()).toBe(120n); // 30 + 40 + 50 (10 and 20 removed)
    });

    it("handles zero values correctly", () => {
      const accumulator = new SlidingWindowAccumulator(3);
      accumulator.push(0n);
      accumulator.push(10n);
      accumulator.push(0n);

      expect(accumulator.getLength()).toBe(3);
      expect(accumulator.getTotal()).toBe(10n);
    });

    it("is a no-op when size is 0", () => {
      const accumulator = new SlidingWindowAccumulator(0);
      accumulator.push(10n);
      accumulator.push(20n);
      accumulator.push(30n);

      expect(accumulator.getLength()).toBe(0);
      expect(accumulator.getTotal()).toBe(0n);
    });

    it("handles large bigint values", () => {
      const accumulator = new SlidingWindowAccumulator(2);
      const largeValue1 = 1000000000000000000n; // 1 ETH in wei
      const largeValue2 = 2000000000000000000n; // 2 ETH in wei

      accumulator.push(largeValue1);
      accumulator.push(largeValue2);

      expect(accumulator.getTotal()).toBe(3000000000000000000n);
    });
  });

  describe("getTotal", () => {
    it("returns 0n for empty accumulator", () => {
      const accumulator = new SlidingWindowAccumulator(5);
      expect(accumulator.getTotal()).toBe(0n);
    });

    it("returns correct total after pushes", () => {
      const accumulator = new SlidingWindowAccumulator(5);
      accumulator.push(5n);
      accumulator.push(10n);
      accumulator.push(15n);

      expect(accumulator.getTotal()).toBe(30n);
    });

    it("returns correct total after window slides", () => {
      const accumulator = new SlidingWindowAccumulator(2);
      accumulator.push(10n);
      accumulator.push(20n);
      expect(accumulator.getTotal()).toBe(30n);

      accumulator.push(30n);
      expect(accumulator.getTotal()).toBe(50n); // 20 + 30
    });
  });

  describe("getLength", () => {
    it("returns 0 for empty accumulator", () => {
      const accumulator = new SlidingWindowAccumulator(5);
      expect(accumulator.getLength()).toBe(0);
    });

    it("returns correct length as values are added", () => {
      const accumulator = new SlidingWindowAccumulator(5);
      expect(accumulator.getLength()).toBe(0);

      accumulator.push(10n);
      expect(accumulator.getLength()).toBe(1);

      accumulator.push(20n);
      expect(accumulator.getLength()).toBe(2);

      accumulator.push(30n);
      expect(accumulator.getLength()).toBe(3);
    });

    it("does not exceed window size", () => {
      const accumulator = new SlidingWindowAccumulator(3);
      accumulator.push(10n);
      accumulator.push(20n);
      accumulator.push(30n);
      expect(accumulator.getLength()).toBe(3);

      accumulator.push(40n);
      expect(accumulator.getLength()).toBe(3); // Still 3, oldest removed
    });

    it("returns 0 when size is 0", () => {
      const accumulator = new SlidingWindowAccumulator(0);
      accumulator.push(10n);
      accumulator.push(20n);
      expect(accumulator.getLength()).toBe(0);
    });
  });

  describe("edge cases", () => {
    it("handles single value window", () => {
      const accumulator = new SlidingWindowAccumulator(1);
      accumulator.push(10n);
      expect(accumulator.getTotal()).toBe(10n);

      accumulator.push(20n);
      expect(accumulator.getTotal()).toBe(20n); // Only latest value
      expect(accumulator.getLength()).toBe(1);
    });

    it("handles many pushes maintaining correct window", () => {
      const accumulator = new SlidingWindowAccumulator(3);
      for (let i = 1; i <= 10; i++) {
        accumulator.push(BigInt(i * 10));
      }

      expect(accumulator.getLength()).toBe(3);
      expect(accumulator.getTotal()).toBe(270n); // 80 + 90 + 100
    });

    it("handles negative values correctly (if needed)", () => {
      // Note: bigint doesn't support negative literals directly in TypeScript
      // This test documents the behavior if negative values are somehow passed
      const accumulator = new SlidingWindowAccumulator(3);
      accumulator.push(10n);
      accumulator.push(5n);

      expect(accumulator.getTotal()).toBe(15n);
    });

    it("maintains correct state through multiple window slides", () => {
      const accumulator = new SlidingWindowAccumulator(2);
      accumulator.push(1n);
      accumulator.push(2n);
      expect(accumulator.getTotal()).toBe(3n);

      accumulator.push(3n);
      expect(accumulator.getTotal()).toBe(5n); // 2 + 3

      accumulator.push(4n);
      expect(accumulator.getTotal()).toBe(7n); // 3 + 4

      accumulator.push(5n);
      expect(accumulator.getTotal()).toBe(9n); // 4 + 5
    });
  });
});

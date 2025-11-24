import { gweiToWei, weiToGwei, weiToGweiNumber } from "../blockchain";
import { WEI_PER_GWEI } from "../../core/constants/blockchain";

describe("weiToGwei", () => {
  it("returns zero when converting zero wei", () => {
    expect(weiToGwei(0n)).toBe(0n);
  });

  it("converts exact multiples of WEI_PER_GWEI to whole gwei", () => {
    expect(weiToGwei(5n * WEI_PER_GWEI)).toBe(5n);
  });

  it("floors fractional gwei values", () => {
    const fractionalWei = 1234n * WEI_PER_GWEI + 567n;
    expect(weiToGwei(fractionalWei)).toBe(1234n);
  });
});

describe("weiToGweiNumber", () => {
  it("returns a number representation of gwei", () => {
    expect(weiToGweiNumber(7n * WEI_PER_GWEI)).toBe(7);
  });

  it("handles large but safe values", () => {
    const gweiValue = 9_000_000n;
    expect(weiToGweiNumber(gweiValue * WEI_PER_GWEI)).toBe(Number(gweiValue));
  });
});

describe("gweiToWei", () => {
  it("converts gwei to wei", () => {
    expect(gweiToWei(42n)).toBe(42n * WEI_PER_GWEI);
  });

  it("is the inverse of weiToGwei for whole gwei amounts", () => {
    const gweiValue = 1_234_567n;
    const wei = gweiToWei(gweiValue);
    expect(weiToGwei(wei)).toBe(gweiValue);
  });
});

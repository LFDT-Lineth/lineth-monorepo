import { isLineaSepolia, isLineaMainnet, isMainnet, isSepolia } from "./chain";

describe("chain", () => {
  describe("isLineaSepolia", () => {
    it("should return true for Linea Sepolia chain ID (59141)", () => {
      expect(isLineaSepolia(59141)).toBe(true);
    });

    it.each([1, 59144, 11155111, 0, 999])("should return false for chain ID %d", (chainId) => {
      expect(isLineaSepolia(chainId)).toBe(false);
    });
  });

  describe("isLineaMainnet", () => {
    it("should return true for Linea Mainnet chain ID (59144)", () => {
      expect(isLineaMainnet(59144)).toBe(true);
    });

    it.each([1, 59141, 11155111, 0, 999])("should return false for chain ID %d", (chainId) => {
      expect(isLineaMainnet(chainId)).toBe(false);
    });
  });

  describe("isMainnet", () => {
    it("should return true for Ethereum Mainnet chain ID (1)", () => {
      expect(isMainnet(1)).toBe(true);
    });

    it.each([59141, 59144, 11155111, 0, 999])("should return false for chain ID %d", (chainId) => {
      expect(isMainnet(chainId)).toBe(false);
    });
  });

  describe("isSepolia", () => {
    it("should return true for Ethereum Sepolia chain ID (11155111)", () => {
      expect(isSepolia(11155111)).toBe(true);
    });

    it.each([1, 59141, 59144, 0, 999])("should return false for chain ID %d", (chainId) => {
      expect(isSepolia(chainId)).toBe(false);
    });
  });
});

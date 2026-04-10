import { getContractsAddressesByChainId } from "./contract";
import {
  L2_MESSAGE_SERVICE_MAINNET_ADDRESS,
  L2_MESSAGE_SERVICE_SEPOLIA_ADDRESS,
  LINEA_ROLLUP_MAINNET_ADDRESS,
  LINEA_ROLLUP_SEPOLIA_ADDRESS,
  TOKEN_BRIDGE_LINEA_MAINNET_ADDRESS,
  TOKEN_BRIDGE_LINEA_SEPOLIA_ADDRESS,
  TOKEN_BRIDGE_MAINNET_ADDRESS,
  TOKEN_BRIDGE_SEPOLIA_ADDRESS,
} from "../constants/address";

describe("contract", () => {
  describe("getContractsAddressesByChainId", () => {
    it("should return mainnet addresses for Ethereum Mainnet (chainId 1)", () => {
      expect(getContractsAddressesByChainId(1)).toEqual({
        messageService: LINEA_ROLLUP_MAINNET_ADDRESS,
        destinationChainMessageService: L2_MESSAGE_SERVICE_MAINNET_ADDRESS,
        tokenBridge: TOKEN_BRIDGE_MAINNET_ADDRESS,
      });
    });

    it("should return sepolia addresses for Ethereum Sepolia (chainId 11155111)", () => {
      expect(getContractsAddressesByChainId(11155111)).toEqual({
        messageService: LINEA_ROLLUP_SEPOLIA_ADDRESS,
        destinationChainMessageService: L2_MESSAGE_SERVICE_SEPOLIA_ADDRESS,
        tokenBridge: TOKEN_BRIDGE_SEPOLIA_ADDRESS,
      });
    });

    it("should return Linea mainnet addresses for Linea Mainnet (chainId 59144)", () => {
      expect(getContractsAddressesByChainId(59144)).toEqual({
        messageService: L2_MESSAGE_SERVICE_MAINNET_ADDRESS,
        destinationChainMessageService: LINEA_ROLLUP_MAINNET_ADDRESS,
        tokenBridge: TOKEN_BRIDGE_LINEA_MAINNET_ADDRESS,
      });
    });

    it("should return Linea sepolia addresses for Linea Sepolia (chainId 59141)", () => {
      expect(getContractsAddressesByChainId(59141)).toEqual({
        messageService: L2_MESSAGE_SERVICE_SEPOLIA_ADDRESS,
        destinationChainMessageService: LINEA_ROLLUP_SEPOLIA_ADDRESS,
        tokenBridge: TOKEN_BRIDGE_LINEA_SEPOLIA_ADDRESS,
      });
    });

    it("should throw for an unsupported chain ID", () => {
      expect(() => getContractsAddressesByChainId(999)).toThrow(
        "Unsupported chain ID. Only Ethereum and Linea networks are supported.",
      );
    });

    it("should throw for chain ID 0", () => {
      expect(() => getContractsAddressesByChainId(0)).toThrow(
        "Unsupported chain ID. Only Ethereum and Linea networks are supported.",
      );
    });
  });
});

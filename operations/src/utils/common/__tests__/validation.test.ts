import { describe, it, expect } from "@jest/globals";
import { ethers } from "ethers";
import { isValidProtocolUrl, validateEthereumAddress, validateHexString, validateUrl } from "../validation.js";

describe("Validation utils", () => {
  describe("validateEthereumAddress", () => {
    it("should throw an error when the input is not a valid Ethereum address", () => {
      const invalidAddress = "0x0a0e";
      expect(() => validateEthereumAddress(invalidAddress, "Address")).toThrow(
        `Address is not a valid Ethereum address.`,
      );
    });

    it("should return the input when it is a valid Ethereum address", () => {
      const address = ethers.hexlify(ethers.randomBytes(20));
      expect(validateEthereumAddress(address, "Address")).toStrictEqual(address);
    });
  });

  describe("isValidProtocolUrl", () => {
    it("should return false when the url is not valid", () => {
      const invalidUrl = "www.test.com";
      expect(isValidProtocolUrl(invalidUrl, ["http:", "https:"])).toBeFalsy();
    });

    it("should return false when the url protocol is not allowed", () => {
      const invalidUrl = "tcp://test.com";
      expect(isValidProtocolUrl(invalidUrl, ["http:", "https:"])).toBeFalsy();
    });

    it("should return true when the url valid", () => {
      const url = "http://test.com";
      expect(isValidProtocolUrl(url, ["http:", "https:"])).toBeTruthy();
    });
  });

  describe("validateUrl", () => {
    it("should throw an error when the input is not a valid url", () => {
      const invalidUrl = "www.test.com";
      expect(() => validateUrl("Url", invalidUrl, ["http:", "https:"])).toThrow(
        `Url, with value: ${invalidUrl} is not a valid URL`,
      );
    });

    it("should return the input when it is a valid url", () => {
      const url = "http://test.com";
      expect(validateUrl("Url", url, ["http:", "https:"])).toStrictEqual(url);
    });
  });

  describe("validateHexString", () => {
    it("should throw an error when the input is not a hex string", () => {
      const invalidHexString = "0a1f";
      expect(() => validateHexString(invalidHexString)).toThrow(`Input must be a hexadecimal string.`);
    });

    it("should return the input when it is a hex string", () => {
      const hexString = "0x0a1f";
      expect(validateHexString(hexString)).toStrictEqual(hexString);
    });
  });
});

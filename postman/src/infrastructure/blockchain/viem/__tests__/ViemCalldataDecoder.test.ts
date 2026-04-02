import { describe, it, expect } from "@jest/globals";
import { encodeFunctionData, parseAbi, decodeFunctionData } from "viem";

import { TestLogger } from "../../../../utils/testing/helpers";
import { ViemCalldataDecoder } from "../ViemCalldataDecoder";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return { ...actual, decodeFunctionData: jest.fn(actual.decodeFunctionData) };
});

describe("ViemCalldataDecoder", () => {
  const logger = new TestLogger(ViemCalldataDecoder.name);
  const decoder = new ViemCalldataDecoder(logger);

  it("returns empty record if the calldata does not match the function signature", () => {
    const abi = parseAbi(["function foo(uint256 bar)"]);
    const calldata = encodeFunctionData({
      abi,
      functionName: "foo",
      args: [123n],
    });

    const result = decoder.decode("function baz(address quux)", calldata);
    expect(result).toEqual({});
  });

  it("returns empty record when calldata is only a 4-byte selector", () => {
    const selectorOnly = "0xdeadbeef";
    const result = decoder.decode("function foo(uint256)", selectorOnly);
    expect(result).toEqual({});
  });

  it("returns empty record when calldata is shorter than a selector", () => {
    const result = decoder.decode("function foo(uint256)", "0xdead");
    expect(result).toEqual({});
  });

  it("returns empty record if the ABI is malformed", () => {
    const malformedAbiString = "function foo(uint256 bar";
    const calldata = "0xdeadbeef0000000000000000000000000000000000000000000000000000000000000001";

    const result = decoder.decode(malformedAbiString, calldata);
    expect(result).toEqual({});
  });

  it("decodes a simple uint256 argument", () => {
    const abi = parseAbi(["function transfer(address to, uint256 amount)"]);
    const calldata = encodeFunctionData({
      abi,
      functionName: "transfer",
      args: ["0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1000n],
    });

    const result = decoder.decode("function transfer(address to, uint256 amount)", calldata);
    expect((result.to as string).toLowerCase()).toBe("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb");
    expect(result.amount).toBe(1000n);
  });

  it("decodes a function with no arguments by returning empty record", () => {
    const abi = parseAbi(["function noArgs()"]);
    const calldata = encodeFunctionData({ abi, functionName: "noArgs" });

    const result = decoder.decode("function noArgs()", calldata);
    expect(result).toEqual({});
  });

  it("decodes bytes calldata argument", () => {
    const abi = parseAbi(["function sendMessage(address to, bytes calldata)"]);
    const calldata = encodeFunctionData({
      abi,
      functionName: "sendMessage",
      args: ["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "0xdeadbeef"],
    });

    const result = decoder.decode("function sendMessage(address to, bytes calldata data)", calldata);
    expect((result.to as string).toLowerCase()).toBe("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa");
    expect(result.data).toBe("0xdeadbeef");
  });

  it("falls back to index-based keys for unnamed parameters", () => {
    const iface = "function foo(uint256, address)";
    const abi = parseAbi([iface]);
    const calldata = encodeFunctionData({
      abi,
      functionName: "foo",
      args: [42n, "0xcccccccccccccccccccccccccccccccccccccccc"],
    });

    const result = decoder.decode(iface, calldata);
    expect(result["0"]).toBe(42n);
    expect((result["1"] as string).toLowerCase()).toBe("0xcccccccccccccccccccccccccccccccccccccccc");
  });

  it("accepts human-readable ABI string matching ethers Interface format", () => {
    // Verify that parseAbi can handle the same format that ethers Interface accepted
    const iface = "function foo(uint256 bar, address baz)";
    const abi = parseAbi([iface]);
    const calldata = encodeFunctionData({
      abi,
      functionName: "foo",
      args: [42n, "0xcccccccccccccccccccccccccccccccccccccccc"],
    });

    const result = decoder.decode(iface, calldata);
    expect(result.bar).toBe(42n);
    expect((result.baz as string).toLowerCase()).toBe("0xcccccccccccccccccccccccccccccccccccccccc");
  });

  it("returns empty record when decodeFunctionData returns undefined args", () => {
    const iface = "function foo(uint256 bar)";
    const abi = parseAbi([iface]);
    const calldata = encodeFunctionData({ abi, functionName: "foo", args: [1n] });

    (decodeFunctionData as jest.Mock).mockReturnValueOnce({ functionName: "foo", args: undefined });

    const result = decoder.decode(iface, calldata);
    expect(result).toEqual({});
  });

  it("returns empty record when decodeFunctionData returns a functionName not in the ABI", () => {
    const iface = "function foo(uint256 bar)";
    const abi = parseAbi([iface]);
    const calldata = encodeFunctionData({ abi, functionName: "foo", args: [1n] });

    (decodeFunctionData as jest.Mock).mockReturnValueOnce({ functionName: "nonExistent", args: [1n] });

    const result = decoder.decode(iface, calldata);
    expect(result).toEqual({});
  });
});

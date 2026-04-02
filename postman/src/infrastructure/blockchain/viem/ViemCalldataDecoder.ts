import { ILogger } from "@consensys/linea-shared-utils";
import { type Abi, type AbiFunction, decodeFunctionData, type Hex, parseAbi } from "viem";

import { ICalldataDecoder } from "../../../core/services/ICalldataDecoder";

type DecodedFunctionCall = { abiFunction: AbiFunction; args: readonly unknown[] };

export class ViemCalldataDecoder implements ICalldataDecoder {
  // "0x" prefix (2 chars) + 4-byte selector (8 hex chars) = 10 characters
  private static readonly SELECTOR_ONLY_LENGTH = 10;

  constructor(private readonly logger: ILogger) {}

  public decode(calldataFunctionInterface: string, calldata: Hex): Record<string, unknown> {
    if (calldata.length <= ViemCalldataDecoder.SELECTOR_ONLY_LENGTH) {
      this.logger.debug("Calldata is selector-only or shorter, skipping decode.", {
        calldataLength: calldata.length,
      });
      return {};
    }

    const decoded = this.tryDecodeFunctionData(calldataFunctionInterface, calldata);
    if (!decoded) return {};

    this.logger.debug("Successfully decoded calldata.", {
      functionName: decoded.abiFunction.name,
    });

    return Object.fromEntries(decoded.abiFunction.inputs.map((input, i) => [input.name || String(i), decoded.args[i]]));
  }

  private tryDecodeFunctionData(calldataFunctionInterface: string, calldata: Hex): DecodedFunctionCall | null {
    try {
      const abi = parseAbi([calldataFunctionInterface]) as unknown as Abi;
      const { functionName, args } = decodeFunctionData({ abi, data: calldata });
      if (!args) return null;

      const abiFunction = abi.find((item) => item.type === "function" && item.name === functionName);
      if (!abiFunction || abiFunction.type !== "function") return null;

      return { abiFunction, args };
    } catch (e) {
      this.logger.warn("Failed to decode calldata against the provided ABI.", {
        calldataFunctionInterface,
        selector: calldata.slice(0, 10),
        error: e,
      });
      return null;
    }
  }
}

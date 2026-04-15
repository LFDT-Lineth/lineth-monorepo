import { describe, expect, it, jest } from "@jest/globals";
import { BaseError } from "viem";

import { isContractRevert } from "./retry";

describe("retry helper", () => {
  it("detects transaction execution contract reverts", () => {
    expect(
      isContractRevert({
        name: "TransactionExecutionError",
        cause: { name: "ContractFunctionRevertedError" },
      }),
    ).toBe(true);
  });

  it("detects contract function execution reverts from viem simulation", () => {
    expect(
      isContractRevert({
        name: "ContractFunctionExecutionError",
        cause: { name: "ContractFunctionRevertedError" },
      }),
    ).toBe(true);
  });

  it("detects nested call execution reverts", () => {
    expect(
      isContractRevert({
        name: "ContractFunctionExecutionError",
        cause: {
          name: "EstimateGasExecutionError",
          cause: { name: "CallExecutionError" },
        },
      }),
    ).toBe(true);
  });

  it("uses viem walk for BaseError chains", () => {
    const revertedError = Object.assign(new BaseError("execution reverted"), {
      name: "ContractFunctionRevertedError",
    });
    const simulationError = Object.assign(new BaseError("simulation failed"), {
      name: "ContractFunctionExecutionError",
    });
    const walkImpl = ((predicate?: (error: unknown) => boolean) =>
      predicate?.(revertedError) ? revertedError : simulationError) as unknown as BaseError["walk"];
    const walk = jest.fn(walkImpl);
    (simulationError as BaseError & { walk: BaseError["walk"] }).walk = walk as unknown as BaseError["walk"];

    expect(isContractRevert(simulationError)).toBe(true);
    expect(walk).toHaveBeenCalled();
  });

  it("ignores non-revert send errors", () => {
    expect(
      isContractRevert({
        name: "TransactionExecutionError",
        cause: { name: "NonceTooLowError" },
      }),
    ).toBe(false);
  });
});

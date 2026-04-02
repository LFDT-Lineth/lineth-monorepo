import { describe, it, expect } from "@jest/globals";
import {
  HttpRequestError,
  TimeoutError,
  ContractFunctionRevertedError,
  RpcRequestError,
  UserRejectedRequestError,
  TransactionRejectedRpcError,
  InternalRpcError,
  LimitExceededRpcError,
  ResourceUnavailableRpcError,
  ContractFunctionExecutionError,
  CallExecutionError,
  ExecutionRevertedError,
  TransactionExecutionError,
  InsufficientFundsError,
  NonceTooLowError,
  NonceTooHighError,
  FeeCapTooLowError,
  IntrinsicGasTooLowError,
  TipAboveFeeCapError,
  BaseError as ViemBaseError,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { DatabaseErrorType, DatabaseRepoName } from "../../../../core/enums";
import { DatabaseAccessError } from "../../../../core/errors/DatabaseErrors";
import { TEST_L1_SIGNER_PRIVATE_KEY } from "../../../../utils/testing/constants";
import { ViemErrorParser } from "../ViemErrorParser";

// RateLimitExceeded() error selector: keccak256("RateLimitExceeded()")[:4]
const RATE_LIMIT_EXCEEDED_SELECTOR = "0xa74c1c5f";
// Some other custom error selector (e.g. "SomeOtherError()")
const OTHER_ERROR_SELECTOR = "0xb5326ac7";

const TEST_ACCOUNT = privateKeyToAccount(TEST_L1_SIGNER_PRIVATE_KEY);

function createRpcRequestError(message: string, data?: string) {
  return new RpcRequestError({
    body: {},
    url: "http://localhost",
    error: { code: -32000, message, ...(data !== undefined && { data }) },
  });
}

describe("ViemErrorParser", () => {
  const parser = new ViemErrorParser();

  describe("DatabaseAccessError", () => {
    it("should return retryable for DatabaseAccessError", () => {
      const error = new DatabaseAccessError(
        DatabaseRepoName.MessageRepository,
        DatabaseErrorType.Read,
        new Error("connection timeout"),
      );
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
      expect(result.message).toContain("connection timeout");
    });
  });

  describe("Non-viem errors", () => {
    it("should return retryable for generic Error", () => {
      const error = new Error("generic error");
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
      expect(result.message).toBe("generic error");
    });

    it("should return retryable for null", () => {
      const result = parser.parse(null);
      expect(result.retryable).toBe(true);
      expect(result.message).toBe("");
    });

    it("should return retryable for undefined", () => {
      const result = parser.parse(undefined);
      expect(result.retryable).toBe(true);
      expect(result.message).toBe("");
    });

    it("should return retryable for string error", () => {
      const result = parser.parse("something went wrong");
      expect(result.retryable).toBe(true);
      expect(result.message).toBe("something went wrong");
    });
  });

  describe("Retryable viem errors", () => {
    it("should return retryable for HttpRequestError", () => {
      const error = new HttpRequestError({ url: "http://localhost" });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for TimeoutError", () => {
      const error = new TimeoutError({ body: {}, url: "http://localhost" });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for InternalRpcError", () => {
      const error = new InternalRpcError(new Error("internal"));
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for LimitExceededRpcError", () => {
      const error = new LimitExceededRpcError(new Error("rate limited"));
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for ResourceUnavailableRpcError", () => {
      const error = new ResourceUnavailableRpcError(new Error("unavailable"));
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });
  });

  describe("Non-retryable viem errors", () => {
    it("should return not retryable for UserRejectedRequestError", () => {
      const error = new UserRejectedRequestError(new Error("rejected"));
      const result = parser.parse(error);
      expect(result.retryable).toBe(false);
    });

    it("should return not retryable for TransactionRejectedRpcError", () => {
      const error = new TransactionRejectedRpcError(new Error("rejected"));
      const result = parser.parse(error);
      expect(result.retryable).toBe(false);
    });
  });

  describe("RpcRequestError with details", () => {
    it("should return not retryable for execution reverted", () => {
      const error = createRpcRequestError("execution reverted");
      const result = parser.parse(error);
      expect(result.retryable).toBe(false);
    });

    it("should return retryable for execution reverted with RateLimitExceeded data", () => {
      const error = createRpcRequestError("execution reverted", RATE_LIMIT_EXCEEDED_SELECTOR);
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return not retryable for execution reverted with other revert data", () => {
      const error = createRpcRequestError("execution reverted", OTHER_ERROR_SELECTOR);
      const result = parser.parse(error);
      expect(result.retryable).toBe(false);
    });

    it("should return retryable for nonce too low", () => {
      const result = parser.parse(createRpcRequestError("nonce too low"));
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for already known", () => {
      const result = parser.parse(createRpcRequestError("already known"));
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for insufficient funds", () => {
      const result = parser.parse(createRpcRequestError("insufficient funds for gas * price + value"));
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for nonce too high", () => {
      const result = parser.parse(createRpcRequestError("nonce too high"));
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for max fee per gas less than block base fee", () => {
      const result = parser.parse(createRpcRequestError("max fee per gas less than block base fee"));
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for intrinsic gas too low", () => {
      const result = parser.parse(createRpcRequestError("intrinsic gas too low"));
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for transaction already imported", () => {
      const result = parser.parse(createRpcRequestError("transaction already imported"));
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for max priority fee per gas higher than max fee per gas", () => {
      const result = parser.parse(createRpcRequestError("max priority fee per gas higher than max fee per gas"));
      expect(result.retryable).toBe(true);
    });

    it("should return not retryable for gas required exceeds allowance", () => {
      const result = parser.parse(createRpcRequestError("gas required exceeds allowance"));
      expect(result.retryable).toBe(false);
    });
  });

  describe("Nested node errors (TransactionExecutionError wrapping node error wrapping RpcRequestError)", () => {
    it("should return retryable for nested InsufficientFundsError", () => {
      const rpcError = createRpcRequestError("insufficient funds for gas * price + value");
      const nodeError = new InsufficientFundsError({ cause: rpcError });
      const error = new TransactionExecutionError(nodeError, {
        account: TEST_ACCOUNT,
        chain: undefined,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for nested NonceTooLowError", () => {
      const rpcError = createRpcRequestError("nonce too low");
      const nodeError = new NonceTooLowError({ cause: rpcError });
      const error = new TransactionExecutionError(nodeError, {
        account: TEST_ACCOUNT,
        chain: undefined,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for nested NonceTooHighError", () => {
      const rpcError = createRpcRequestError("nonce too high");
      const nodeError = new NonceTooHighError({ cause: rpcError });
      const error = new TransactionExecutionError(nodeError, {
        account: TEST_ACCOUNT,
        chain: undefined,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for nested FeeCapTooLowError", () => {
      const rpcError = createRpcRequestError("max fee per gas less than block base fee");
      const nodeError = new FeeCapTooLowError({ cause: rpcError });
      const error = new TransactionExecutionError(nodeError, {
        account: TEST_ACCOUNT,
        chain: undefined,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for nested IntrinsicGasTooLowError", () => {
      const rpcError = createRpcRequestError("intrinsic gas too low");
      const nodeError = new IntrinsicGasTooLowError({ cause: rpcError });
      const error = new TransactionExecutionError(nodeError, {
        account: TEST_ACCOUNT,
        chain: undefined,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for nested TipAboveFeeCapError", () => {
      const rpcError = createRpcRequestError("max priority fee per gas higher than max fee per gas");
      const nodeError = new TipAboveFeeCapError({ cause: rpcError });
      const error = new TransactionExecutionError(nodeError, {
        account: TEST_ACCOUNT,
        chain: undefined,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return not retryable for nested ExecutionRevertedError", () => {
      const rpcError = createRpcRequestError("execution reverted");
      const nodeError = new ExecutionRevertedError({ cause: rpcError, message: "execution reverted" });
      const error = new TransactionExecutionError(nodeError, {
        account: TEST_ACCOUNT,
        chain: undefined,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(false);
    });

    it("should return retryable for nested ExecutionRevertedError with RateLimitExceeded data", () => {
      const rpcError = createRpcRequestError("execution reverted", RATE_LIMIT_EXCEEDED_SELECTOR);
      const nodeError = new ExecutionRevertedError({
        cause: rpcError,
        message: "execution reverted: RateLimitExceeded",
      });
      const error = new TransactionExecutionError(nodeError, {
        account: TEST_ACCOUNT,
        chain: undefined,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });
  });

  describe("ContractFunctionRevertedError (nested)", () => {
    it("should return not retryable for contract revert via ContractFunctionExecutionError", () => {
      const revertError = new ContractFunctionRevertedError({
        abi: [],
        functionName: "test",
      });
      const executionError = new ContractFunctionExecutionError(revertError, {
        abi: [],
        functionName: "test",
      });
      const result = parser.parse(executionError);
      expect(result.retryable).toBe(false);
    });

    it("should return retryable for contract revert with RateLimitExceeded data", () => {
      const revertError = new ContractFunctionRevertedError({
        abi: [],
        functionName: "test",
        data: RATE_LIMIT_EXCEEDED_SELECTOR,
      });
      const executionError = new ContractFunctionExecutionError(revertError, {
        abi: [],
        functionName: "test",
      });
      const result = parser.parse(executionError);
      expect(result.retryable).toBe(true);
    });

    it("should return not retryable for contract revert with other revert data", () => {
      const revertError = new ContractFunctionRevertedError({
        abi: [],
        functionName: "test",
        data: OTHER_ERROR_SELECTOR,
      });
      const executionError = new ContractFunctionExecutionError(revertError, {
        abi: [],
        functionName: "test",
      });
      const result = parser.parse(executionError);
      expect(result.retryable).toBe(false);
    });
  });

  describe("ExecutionRevertedError nested in CallExecutionError", () => {
    it("should return not retryable for ExecutionRevertedError in CallExecutionError", () => {
      const rpcError = createRpcRequestError("execution reverted");
      const revertError = new ExecutionRevertedError({ cause: rpcError, message: "execution reverted" });
      const error = new CallExecutionError(revertError, {
        account: TEST_ACCOUNT,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(false);
    });

    it("should return retryable for ExecutionRevertedError with RateLimitExceeded in CallExecutionError", () => {
      const rpcError = createRpcRequestError("execution reverted", RATE_LIMIT_EXCEEDED_SELECTOR);
      const revertError = new ExecutionRevertedError({
        cause: rpcError,
        message: "execution reverted: RateLimitExceeded",
      });
      const error = new CallExecutionError(revertError, {
        account: TEST_ACCOUNT,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });

    it("should return retryable for InsufficientFundsError in CallExecutionError", () => {
      const rpcError = createRpcRequestError("insufficient funds for gas * price + value");
      const nodeError = new InsufficientFundsError({ cause: rpcError });
      const error = new CallExecutionError(nodeError, {
        account: TEST_ACCOUNT,
      });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });
  });

  describe("ExecutionRevertedError without RpcRequestError wrapping", () => {
    it("should return not retryable for bare ExecutionRevertedError", () => {
      const execReverted = new ExecutionRevertedError({
        cause: new ViemBaseError("inner"),
        message: "execution reverted",
      });
      const error = new ViemBaseError("wrapped", { cause: execReverted });
      const result = parser.parse(error);
      expect(result.retryable).toBe(false);
    });

    it("should return retryable for bare ExecutionRevertedError with RateLimitExceeded in message", () => {
      const execReverted = new ExecutionRevertedError({
        cause: new ViemBaseError("inner"),
        message: "execution reverted: RateLimitExceeded",
      });
      const error = new ViemBaseError("RateLimitExceeded", { cause: execReverted });
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
    });
  });

  describe("Default viem error", () => {
    it("should return retryable for unknown ViemBaseError", () => {
      const error = new ViemBaseError("unknown viem error");
      const result = parser.parse(error);
      expect(result.retryable).toBe(true);
      expect(result.message).toBe("unknown viem error");
    });
  });

  describe("Message extraction", () => {
    it("should use shortMessage when available", () => {
      const error = new HttpRequestError({ url: "http://localhost" });
      const result = parser.parse(error);
      expect(result.message).toBe(error.shortMessage || error.message);
    });

    it("should fall back to error.message when shortMessage is empty", () => {
      const error = new ViemBaseError("test message");
      Object.defineProperty(error, "shortMessage", { value: "", configurable: true });
      const result = parser.parse(error);
      expect(result.message).toBe(error.message);
      expect(result.message).not.toBe("");
    });
  });
});

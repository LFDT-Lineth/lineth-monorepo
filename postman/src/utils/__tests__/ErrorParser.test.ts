import { makeBaseError } from "@consensys/linea-sdk";
import { describe, it, expect } from "@jest/globals";
import { ErrorCode, makeError } from "ethers";

import { DatabaseErrorType, DatabaseRepoName } from "../../core/enums";
import { DatabaseAccessError } from "../../core/errors";
import { ErrorParser } from "../ErrorParser";
import { generateMessage } from "../testing/helpers";

describe("ErrorParser", () => {
  describe("parseErrorWithMitigation", () => {
    it("should return null when error is null", () => {
      expect(ErrorParser.parseErrorWithMitigation(null)).toStrictEqual(null);
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when error is not instance of BaseError", () => {
      expect(ErrorParser.parseErrorWithMitigation(new Error("any reason"))).toStrictEqual({
        errorMessage: "any reason",
        errorCode: "UNKNOWN_ERROR",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when error is instance of BaseError but the underlying error is not an EthersError", () => {
      expect(ErrorParser.parseErrorWithMitigation(makeBaseError(new Error("any reason")))).toStrictEqual({
        errorMessage: "any reason",
        errorCode: "UNKNOWN_ERROR",
        mitigation: {
          shouldRetry: false,
        },
      });
    });
  });

  describe("parseEthersError", () => {
    it("should return NETWORK_ERROR and shouldRetry = true when error code = NETWORK_ERROR", () => {
      const error = makeError("any reason", "NETWORK_ERROR");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason (code=NETWORK_ERROR, version=6.13.7)",
        errorCode: "NETWORK_ERROR",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return SERVER_ERROR and shouldRetry = true when error code = SERVER_ERROR", () => {
      const error = makeError("any reason", "SERVER_ERROR");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason (code=SERVER_ERROR, version=6.13.7)",
        errorCode: "SERVER_ERROR",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return TIMEOUT and shouldRetry = true when error code = TIMEOUT", () => {
      const error = makeError("any reason", "TIMEOUT");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason (code=TIMEOUT, version=6.13.7)",
        errorCode: "TIMEOUT",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return INSUFFICIENT_FUNDS and shouldRetry = true when error code = INSUFFICIENT_FUNDS", () => {
      const error = makeError("any reason", "INSUFFICIENT_FUNDS");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason (code=INSUFFICIENT_FUNDS, version=6.13.7)",
        errorCode: "INSUFFICIENT_FUNDS",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return REPLACEMENT_UNDERPRICED and shouldRetry = true when error code = REPLACEMENT_UNDERPRICED", () => {
      const error = makeError("any reason", "REPLACEMENT_UNDERPRICED");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason (code=REPLACEMENT_UNDERPRICED, version=6.13.7)",
        errorCode: "REPLACEMENT_UNDERPRICED",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return NONCE_EXPIRED and shouldRetry = true when error code = NONCE_EXPIRED", () => {
      const error = makeError("any reason", "NONCE_EXPIRED");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason (code=NONCE_EXPIRED, version=6.13.7)",
        errorCode: "NONCE_EXPIRED",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION", () => {
      const error = makeError("any reason", "CALL_EXCEPTION");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = false when error code = CALL_EXCEPTION with short message as execution reverted", () => {
      const error = makeError("execution reverted", "CALL_EXCEPTION", {
        info: {
          error: {
            message: "execution reverted for some reason",
          },
        },
        reason: "execution reverted",
        action: "call",
        data: "0x0123456789abcdef",
        transaction: {
          to: null,
          from: undefined,
          data: "",
        },
        invocation: null,
        revert: null,
      });

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "execution reverted for some reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = false when error code = CALL_EXCEPTION with inner error code as 4001", () => {
      const error = makeError("any reason", "CALL_EXCEPTION", {
        info: {
          error: {
            message: "execution reverted for some reason",
            code: 4001,
          },
        },
        action: "call",
        data: null,
        reason: null,
        transaction: {
          to: null,
          from: undefined,
          data: "",
        },
        invocation: null,
        revert: null,
      });

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "execution reverted for some reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = false when error code = CALL_EXCEPTION with inner error code as -32603", () => {
      const error = makeError("any reason", "CALL_EXCEPTION", {
        info: {
          error: {
            message: "execution reverted for some reason",
            code: -32603,
          },
        },
        action: "call",
        data: null,
        reason: null,
        transaction: {
          to: null,
          from: undefined,
          data: "",
        },
        invocation: null,
        revert: null,
      });

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "execution reverted for some reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION and inner error code as -32000 and message as gas required exceeds allowance (0)", () => {
      const error = makeError("any reason", "CALL_EXCEPTION", {
        info: {
          error: {
            message: "gas required exceeds allowance (0)",
            code: -32000,
          },
        },
        action: "call",
        data: null,
        reason: null,
        transaction: {
          to: null,
          from: undefined,
          data: "",
        },
        invocation: null,
        revert: null,
      });

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "gas required exceeds allowance (0)",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION and inner error code as -32000 and message as max priority fee per gas higher", () => {
      const error = makeError("any reason", "CALL_EXCEPTION", {
        info: {
          error: {
            message: "max priority fee per gas higher than max fee per gas",
            code: -32000,
          },
        },
        action: "call",
        data: null,
        reason: null,
        transaction: {
          to: null,
          from: undefined,
          data: "",
        },
        invocation: null,
        revert: null,
      });

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "max priority fee per gas higher than max fee per gas",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION and inner error code as -32000 and message as max fee per gas less than block base fee", () => {
      const error = makeError("any reason", "CALL_EXCEPTION", {
        info: {
          error: {
            message: "max fee per gas less than block base fee",
            code: -32000,
          },
        },
        action: "call",
        data: null,
        reason: null,
        transaction: {
          to: null,
          from: undefined,
          data: "",
        },
        invocation: null,
        revert: null,
      });

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "max fee per gas less than block base fee",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION with other inner error", () => {
      const error = makeError("any reason", "CALL_EXCEPTION", {
        info: {
          error: {
            message: "invalid method parameters",
            code: -32602,
          },
        },
        action: "call",
        data: null,
        reason: null,
        transaction: {
          to: null,
          from: undefined,
          data: "",
        },
        invocation: null,
        revert: null,
      });

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return ACTION_REJECTED and shouldRetry = false when error code = ACTION_REJECTED", () => {
      const error = makeError("any reason", "ACTION_REJECTED");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason",
        errorCode: "ACTION_REJECTED",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when error code = UNKNOWN_ERROR", () => {
      const error = makeError("any reason", "UNKNOWN_ERROR");

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "any reason",
        errorCode: "UNKNOWN_ERROR",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when error = GasEstimationError", () => {
      const gasEstimationError = makeBaseError(makeError("Gas estimation failed", "UNKNOWN_ERROR"), generateMessage());

      expect(ErrorParser.parseErrorWithMitigation(gasEstimationError)).toStrictEqual({
        errorMessage: "Gas estimation failed",
        errorCode: "UNKNOWN_ERROR",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when error is execution reverted", () => {
      const error = makeError("Gas estimation failed", "UNKNOWN_ERROR", {
        error: {
          code: -32000,
          message: "execution reverted",
          data: "0x0123456789abcdef",
        },
      });

      expect(ErrorParser.parseEthersError(error)).toStrictEqual({
        errorMessage: "execution reverted",
        errorCode: "UNKNOWN_ERROR",
        data: "0x0123456789abcdef",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when error = DatabaseAccessError", () => {
      const databaseAccessError = new DatabaseAccessError(
        DatabaseRepoName.MessageRepository,
        DatabaseErrorType.Insert,
        new Error("Database access failed"),
        generateMessage(),
      );

      expect(ErrorParser.parseErrorWithMitigation(databaseAccessError)).toStrictEqual({
        errorMessage: "MessageRepository: insert - Database access failed",
        errorCode: "UNKNOWN_ERROR",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when errors are other types", () => {
      const otherErrorCodes: ErrorCode[] = [
        "NOT_IMPLEMENTED",
        "UNSUPPORTED_OPERATION",
        "BAD_DATA",
        "CANCELLED",
        "BUFFER_OVERRUN",
        "NUMERIC_FAULT",
        "INVALID_ARGUMENT",
        "MISSING_ARGUMENT",
        "UNEXPECTED_ARGUMENT",
        "VALUE_MISMATCH",
        "TRANSACTION_REPLACED",
        "UNCONFIGURED_NAME",
        "OFFCHAIN_FAULT",
      ];
      otherErrorCodes.forEach((errorCode: ErrorCode) => {
        const error = makeError("any reason", errorCode);

        expect(ErrorParser.parseEthersError(error)).toStrictEqual({
          errorMessage: "any reason",
          errorCode: errorCode,
          mitigation: {
            shouldRetry: true,
          },
        });
      });
    });
  });
});

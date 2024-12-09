import { describe, it, expect } from "@jest/globals";
import { ErrorCode, EthersError } from "ethers";
import { GasEstimationError } from "@consensys/linea-sdk";
import { ErrorParser } from "../ErrorParser";
import { DatabaseAccessError } from "../../core/errors";
import { DatabaseErrorType, DatabaseRepoName } from "../../core/enums";
import { generateMessage } from "../testing/helpers";

describe("ErrorParser", () => {
  describe("parseErrorWithMitigation", () => {
    it("should return null when error is null", () => {
      expect(ErrorParser.parseErrorWithMitigation(null as unknown as EthersError)).toStrictEqual(null);
    });

    it("should return NETWORK_ERROR and shouldRetry = true when error code = NETWORK_ERROR", () => {
      const errorMessage = {
        code: "NETWORK_ERROR",
        shortMessage: "any reason",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "NETWORK_ERROR",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return SERVER_ERROR and shouldRetry = true when error code = SERVER_ERROR", () => {
      const errorMessage = {
        code: "SERVER_ERROR",
        shortMessage: "any reason",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "SERVER_ERROR",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return TIMEOUT and shouldRetry = true when error code = TIMEOUT", () => {
      const errorMessage = {
        code: "TIMEOUT",
        shortMessage: "any reason",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "TIMEOUT",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return INSUFFICIENT_FUNDS and shouldRetry = true when error code = INSUFFICIENT_FUNDS", () => {
      const errorMessage = {
        code: "INSUFFICIENT_FUNDS",
        shortMessage: "any reason",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "INSUFFICIENT_FUNDS",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return REPLACEMENT_UNDERPRICED and shouldRetry = true when error code = NETWOREPLACEMENT_UNDERPRICEDK_ERROR", () => {
      const errorMessage = {
        code: "REPLACEMENT_UNDERPRICED",
        shortMessage: "any reason",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "REPLACEMENT_UNDERPRICED",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return NONCE_EXPIRED and shouldRetry = true when error code = NONCE_EXPIRED", () => {
      const errorMessage = {
        code: "NONCE_EXPIRED",
        shortMessage: "any reason",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "NONCE_EXPIRED",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "CALL_EXCEPTION",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = false when error code = CALL_EXCEPTION with short message as execution reverted", () => {
      const errorMessage = {
        shortMessage: "execution reverted",
        code: "CALL_EXCEPTION",
        info: {
          error: {
            message: "execution reverted for some reason",
          },
        },
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "execution reverted for some reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = false when error code = CALL_EXCEPTION with inner error code as 4001", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "CALL_EXCEPTION",
        info: {
          error: {
            message: "execution reverted for some reason",
            code: 4001,
          },
        },
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "execution reverted for some reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = false when error code = CALL_EXCEPTION with inner error code as -32603", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "CALL_EXCEPTION",
        info: {
          error: {
            message: "execution reverted for some reason",
            code: -32603,
          },
        },
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "execution reverted for some reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION and inner error code as -32000 and message as gas required exceeds allowance (0)", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "CALL_EXCEPTION",
        info: {
          error: {
            message: "gas required exceeds allowance (0)",
            code: -32000,
          },
        },
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "gas required exceeds allowance (0)",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION and inner error code as -32000 and message as max priority fee per gas higher", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "CALL_EXCEPTION",
        info: {
          error: {
            message: "max priority fee per gas higher than max fee per gas",
            code: -32000,
          },
        },
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "max priority fee per gas higher than max fee per gas",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION and inner error code as -32000 and message as max fee per gas less than block base fee", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "CALL_EXCEPTION",
        info: {
          error: {
            message: "max fee per gas less than block base fee",
            code: -32000,
          },
        },
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "max fee per gas less than block base fee",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return CALL_EXCEPTION and shouldRetry = true when error code = CALL_EXCEPTION with other inner error", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "CALL_EXCEPTION",
        info: {
          error: {
            message: "invalid method parameters",
            code: -32602,
          },
        },
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "CALL_EXCEPTION",
        mitigation: {
          shouldRetry: true,
        },
      });
    });

    it("should return ACTION_REJECTED and shouldRetry = false when error code = ACTION_REJECTED", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "ACTION_REJECTED",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "ACTION_REJECTED",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when error code = UNKNOWN_ERROR", () => {
      const errorMessage = {
        shortMessage: "any reason",
        code: "UNKNOWN_ERROR",
      };

      expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
        context: "any reason",
        errorCode: "UNKNOWN_ERROR",
        mitigation: {
          shouldRetry: false,
        },
      });
    });

    it("should return UNKNOWN_ERROR and shouldRetry = false when error = GasEstimationError", () => {
      const gasEstimationError = new GasEstimationError("Gas estimation failed", generateMessage());

      expect(ErrorParser.parseErrorWithMitigation(gasEstimationError as unknown as EthersError)).toStrictEqual({
        context: "Gas estimation failed",
        errorCode: "UNKNOWN_ERROR",
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

      expect(ErrorParser.parseErrorWithMitigation(databaseAccessError as unknown as EthersError)).toStrictEqual({
        context: "MessageRepository: insert - Database access failed",
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
        const errorMessage = {
          shortMessage: "any reason",
          code: errorCode,
        };

        expect(ErrorParser.parseErrorWithMitigation(errorMessage as unknown as EthersError)).toStrictEqual({
          context: "any reason",
          errorCode: errorCode,
          mitigation: {
            shouldRetry: true,
          },
        });
      });
    });
  });
});

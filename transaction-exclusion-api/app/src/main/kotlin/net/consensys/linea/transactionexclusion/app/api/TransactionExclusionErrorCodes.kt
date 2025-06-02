package net.consensys.linea.transactionexclusion.app.api

import net.consensys.linea.jsonrpc.JsonRpcError
import net.consensys.linea.transactionexclusion.ErrorType
import net.consensys.linea.transactionexclusion.TransactionExclusionError

enum class TransactionExclusionErrorCodes(val code: Int, val message: String) {
  // App/System/Server' error codes
  SERVER_ERROR(-32000, "Server error"),
  ;

  fun toErrorObject(data: Any? = null): JsonRpcError {
    return JsonRpcError(this.code, this.message, data)
  }
}

fun jsonRpcError(appError: TransactionExclusionError): JsonRpcError {
  return when (appError.errorType) {
    ErrorType.SERVER_ERROR ->
      TransactionExclusionErrorCodes.SERVER_ERROR.toErrorObject(appError.errorDetail)
  }
}

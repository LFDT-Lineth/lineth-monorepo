package net.consensys.linea.jsonrpc

import com.fasterxml.jackson.annotation.JsonInclude
import com.fasterxml.jackson.annotation.JsonProperty
import com.fasterxml.jackson.annotation.JsonPropertyOrder

enum class JsonRpcErrorCode(val code: Int, val message: String) {
  PARSE_ERROR(-32700, "Parse error"),
  INVALID_REQUEST(-32600, "Invalid Request"),
  METHOD_NOT_FOUND(-32601, "Method not found"),
  INVALID_PARAMS(-32602, "Invalid params"),
  INTERNAL_ERROR(-32603, "Internal error"),
  UNAUTHORIZED(-40100, "Unauthorized");

  fun toErrorObject(data: Any? = null): JsonRpcError {
    return JsonRpcError(this.code, this.message, data)
  }
}

abstract class JsonRpcResponse(open val jsonrpc: String = "2.0", open val id: Any?) {
  init {
    require(id == null || id is String || id is Int || id is Long) {
      "It must be one of {String, Integer, Long, null}."
    }
  }
}

@JsonPropertyOrder("jsonrpc", "id", "result")
data class JsonRpcSuccessResponse(
  override val jsonrpc: String,
  override val id: Any,
  val result: Any?
) : JsonRpcResponse(jsonrpc, id) {
  constructor(id: Any, result: Any?) : this("2.0", id, result)
  constructor(request: JsonRpcRequest, result: Any?) : this(request.jsonrpc, id = request.id, result)
}

@JsonPropertyOrder("jsonrpc", "id", "error")
@JsonInclude(JsonInclude.Include.NON_NULL)
data class JsonRpcErrorResponse(
  @JsonProperty("id") override val id: Any?,
  @JsonProperty("error") val error: JsonRpcError
) : JsonRpcResponse(id = id) {

  companion object {
    fun parseError(): JsonRpcErrorResponse {
      return JsonRpcErrorResponse(null, JsonRpcErrorCode.PARSE_ERROR.toErrorObject())
    }

    fun invalidRequest(): JsonRpcErrorResponse {
      return JsonRpcErrorResponse(null, JsonRpcErrorCode.INVALID_REQUEST.toErrorObject())
    }

    fun methodNotFound(id: Any, data: Any?): JsonRpcErrorResponse {
      return JsonRpcErrorResponse(id, JsonRpcErrorCode.METHOD_NOT_FOUND.toErrorObject(data))
    }

    fun unauthorized(id: Any): JsonRpcErrorResponse {
      return JsonRpcErrorResponse(id, JsonRpcError.unauthorized())
    }

    fun internalError(id: Any, data: Any?): JsonRpcErrorResponse {
      return JsonRpcErrorResponse(id, JsonRpcError.internalError(data))
    }

    fun invalidParams(id: Any, message: String?): JsonRpcErrorResponse {
      return JsonRpcErrorResponse(id, JsonRpcError.invalidMethodParameter(message))
    }
  }
}

@JsonPropertyOrder("code", "message", "data")
@JsonInclude(JsonInclude.Include.NON_NULL)
data class JsonRpcError(
  @JsonProperty("code") val code: Int,
  @JsonProperty("message") val message: String,
  @JsonProperty("data") val data: Any? = null
) {
  // inlining for better stacktrace
  @Suppress("NOTHING_TO_INLINE")
  inline fun asException() = JsonRpcErrorResponseException(code, message, data)

  companion object {
    @JvmStatic
    fun invalidMethodParameter(message: String?): JsonRpcError =
      JsonRpcError(
        JsonRpcErrorCode.INVALID_PARAMS.code,
        message ?: JsonRpcErrorCode.INVALID_PARAMS.message
      )

    @JvmStatic
    fun invalidMethodParameter(message: String, data: Any): JsonRpcError =
      JsonRpcError(JsonRpcErrorCode.INVALID_PARAMS.code, message, data)

    @JvmStatic
    fun internalError(): JsonRpcError = JsonRpcErrorCode.INTERNAL_ERROR.toErrorObject()

    @JvmStatic
    fun internalError(data: Any?): JsonRpcError =
      JsonRpcErrorCode.INTERNAL_ERROR.toErrorObject(data)

    @JvmStatic
    fun unauthorized(): JsonRpcError = JsonRpcErrorCode.UNAUTHORIZED.toErrorObject()
  }
}

class JsonRpcErrorException(
  override val message: String?,
  val httpStatusCode: Int? = null
) : RuntimeException(message)

class JsonRpcErrorResponseException(
  val rpcErrorCode: Int,
  val rpcErrorMessage: String,
  val rpcErrorData: Any? = null
) : RuntimeException("code=$rpcErrorCode message=$rpcErrorMessage errorData=$rpcErrorData") {
  fun asJsonRpcError(): JsonRpcError = JsonRpcError(rpcErrorCode, rpcErrorMessage, rpcErrorData)
}

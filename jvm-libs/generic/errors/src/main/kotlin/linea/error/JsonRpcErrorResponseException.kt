package linea.error

class JsonRpcErrorResponseException(
  val rpcErrorCode: Int,
  val rpcErrorMessage: String,
  val rpcErrorData: Any? = null,
  val method: String? = null,
) : RuntimeException(
  "${method?.let { "$it failed with JsonRpcError: " }}" +
    "code=$rpcErrorCode message=$rpcErrorMessage errorData=$rpcErrorData",
)

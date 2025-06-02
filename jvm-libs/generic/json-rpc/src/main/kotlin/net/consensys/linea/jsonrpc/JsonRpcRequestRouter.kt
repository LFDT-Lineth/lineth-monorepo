package net.consensys.linea.jsonrpc

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User

class JsonRpcRequestRouter(private val methodHandlers: Map<String, JsonRpcRequestHandler>) :
  JsonRpcRequestHandler {
  override fun invoke(
    user: User?,
    jsonRpcRequest: JsonRpcRequest,
    requestJson: JsonObject,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val handler =
      methodHandlers[jsonRpcRequest.method]
        ?: return Future.succeededFuture(
          Err(JsonRpcErrorResponse.methodNotFound(jsonRpcRequest.id, jsonRpcRequest.method)),
        )

    return handler.invoke(user, jsonRpcRequest, requestJson)
  }
}

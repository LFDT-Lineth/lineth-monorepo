package net.consensys.zkevm.coordinator.api.requesthandlers

import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcRequestHandler
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse

/**
 * JSON-RPC: signals that target checkpoint pause may resume when [linea.coordinator.config.v2.ConflationConfig.ProofAggregation.waitApiResumeAfterTargetBlock] is enabled.
 */
class ConflationTargetCheckpointResumeRequestHandler(
  private val signalResume: () -> Boolean,
) : JsonRpcRequestHandler {
  companion object {
    const val METHOD_NAME = "conflation_signalTargetCheckpointResume"
  }

  override fun invoke(
    user: User?,
    request: JsonRpcRequest,
    requestJson: JsonObject,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val accepted = signalResume()
    return Future.succeededFuture(
      Ok(
        JsonRpcSuccessResponse(
          id = request.id,
          result = accepted,
        ),
      ),
    )
  }
}

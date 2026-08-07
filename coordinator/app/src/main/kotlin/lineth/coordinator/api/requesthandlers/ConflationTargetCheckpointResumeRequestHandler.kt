package lineth.coordinator.api.requesthandlers

import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User
import lineth.jsonrpc.JsonRpcErrorResponse
import lineth.jsonrpc.JsonRpcRequest
import lineth.jsonrpc.JsonRpcRequestHandler
import lineth.jsonrpc.JsonRpcSuccessResponse

/**
 * JSON-RPC: signals that target checkpoint pause may resume when [lineth.coordinator.config.v2.ConflationConfig.ProofAggregation.waitApiResumeAfterTargetBlock] is enabled.
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

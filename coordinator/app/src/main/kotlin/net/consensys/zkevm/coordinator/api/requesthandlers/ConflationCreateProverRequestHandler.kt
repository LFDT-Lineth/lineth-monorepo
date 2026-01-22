package net.consensys.zkevm.coordinator.api.requesthandlers

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcRequestHandler
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.zkevm.coordinator.api.dto.ConflationCreateProverRequestJsonDto
import net.consensys.zkevm.coordinator.app.conflationbacktesting.ConflationBacktestingService

class ConflationCreateProverRequestHandler(private val conflationBacktestingService: ConflationBacktestingService) :
  JsonRpcRequestHandler {
  companion object {
    val METHOD_NAME = "conflation_createProverRequests"
  }

  override fun invoke(
    user: User?,
    request: JsonRpcRequest,
    requestJson: JsonObject,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val createProverRequestJsonDtoList = try {
      ConflationCreateProverRequestJsonDto.parseFrom(request)
    } catch (e: Exception) {
      return Future.succeededFuture(
        Err(
          JsonRpcErrorResponse.invalidParams(
            request.id,
            "Invalid request parameters: ${e.message}",
          ),
        ),
      )
    }
    val jobIds = createProverRequestJsonDtoList.map { dto ->
      conflationBacktestingService.submitConflationBacktestingJob(
        ConflationCreateProverRequestJsonDto.toDomainObject(dto),
      )
    }
    return Future.succeededFuture(
      Ok(
        JsonRpcSuccessResponse(
          id = request.id,
          result = jobIds,
        ),
      ),
    )
  }
}

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
import net.consensys.zkevm.coordinator.app.conflationbacktesting.ConflationBacktestingService

class ConflationGetJobStatusRequestHandler(private val conflationBacktestingService: ConflationBacktestingService) :
  JsonRpcRequestHandler {

  override fun invoke(
    user: User?,
    request: JsonRpcRequest,
    requestJson: JsonObject,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val jobIds = try {
      parseJobIdsFromRequest(request)
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
    val jobStatuses = jobIds.map { jobId -> conflationBacktestingService.getConflationBacktestingJobStatus(jobId).name }
    return Future.succeededFuture(
      Ok(
        JsonRpcSuccessResponse(
          id = request.id,
          result = jobStatuses,
        ),
      ),
    )
  }

  companion object {
    val METHOD_NAME = "conflation_getReconflationJobsStatus"

    fun parseJobIdsFromRequest(request: JsonRpcRequest): List<String> {
      val params = request.params
      return when (params) {
        is List<*> -> {
          try {
            params.map { it as String }
          } catch (e: Exception) {
            throw IllegalArgumentException("Invalid request parameters: all job IDs must be strings")
          }
        }
        else -> {
          throw IllegalArgumentException("Invalid request parameters: expected a list of job IDs")
        }
      }
    }
  }
}

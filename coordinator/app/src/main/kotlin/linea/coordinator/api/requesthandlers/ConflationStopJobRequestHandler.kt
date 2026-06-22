package linea.coordinator.api.requesthandlers

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User
import linea.coordinator.app.conflationbacktesting.ConflationBacktestingService
import net.consensys.linea.async.toVertxFuture
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcRequestHandler
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse

/**
 * JSON-RPC: stops a single in-progress conflation backtesting job.
 *
 * Params: a JSON array containing **exactly one** job id string.
 *
 * Result: `"STOPPED"` on success, or `"ERROR: <message>"` when the job could not be stopped (unknown
 * id, already completed, or a failure during shutdown).
 */
class ConflationStopJobRequestHandler(private val conflationBacktestingService: ConflationBacktestingService) :
  JsonRpcRequestHandler {

  override fun invoke(
    user: User?,
    request: JsonRpcRequest,
    requestJson: JsonObject,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val jobId = try {
      parseJobIdFromRequest(request)
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

    return try {
      conflationBacktestingService.stopConflationBacktestingJob(jobId)
        .handle { _, error ->
          if (error == null) STOPPED_RESULT else "ERROR: ${error.message ?: error.javaClass.simpleName}"
        }
        .thenApply<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> { outcome ->
          Ok(
            JsonRpcSuccessResponse(
              id = request.id,
              result = outcome,
            ),
          )
        }
        .toVertxFuture()
    } catch (e: Exception) {
      Future.succeededFuture(
        Ok(
          JsonRpcSuccessResponse(
            id = request.id,
            result = "ERROR: ${e.message ?: e.javaClass.simpleName}",
          ),
        ),
      )
    }
  }

  companion object {
    const val METHOD_NAME = "conflation_stopReconflationJob"
    const val STOPPED_RESULT = "STOPPED"

    fun parseJobIdFromRequest(request: JsonRpcRequest): String {
      val params = request.params
      if (params !is List<*>) {
        throw IllegalArgumentException("Invalid request parameters: expected a JSON array with exactly one job ID")
      }
      if (params.size != 1) {
        throw IllegalArgumentException(
          "Invalid request parameters: expected exactly one job ID, got ${params.size}",
        )
      }
      return try {
        params[0] as String
      } catch (e: Exception) {
        throw IllegalArgumentException("Invalid request parameters: job ID must be a string", e)
      }
    }
  }
}

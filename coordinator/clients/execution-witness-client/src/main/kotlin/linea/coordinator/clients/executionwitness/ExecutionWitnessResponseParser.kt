package linea.coordinator.clients.executionwitness

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.json.JsonArray
import io.vertx.core.json.JsonObject
import linea.error.ErrorResponse
import linea.executionwitness.ExecutionWitness
import linea.executionwitness.ExecutionWitnessError
import linea.kotlin.decodeHex
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse

object ExecutionWitnessResponseParser {

  fun mapRpcError(jsonRpcErrorResponse: JsonRpcErrorResponse): ErrorResponse<ExecutionWitnessError> {
    return ErrorResponse(
      ExecutionWitnessError.RPC_ERROR,
      "${jsonRpcErrorResponse.error.code}: ${jsonRpcErrorResponse.error.message}",
    )
  }

  fun parseExecutionWitness(
    jsonRpcResponse: JsonRpcSuccessResponse,
  ): Result<ExecutionWitness, ErrorResponse<ExecutionWitnessError>> {
    if (jsonRpcResponse.result == null) {
      return Err(
        ErrorResponse(
          ExecutionWitnessError.NULL_RESULT,
          "debug_executionWitness returned null (witness unavailable for block)",
        ),
      )
    }

    return try {
      val json = jsonRpcResponse.result as JsonObject
      Ok(
        ExecutionWitness(
          state = parseHexList(json, "state"),
          keys = parseHexList(json, "keys"),
          codes = parseHexList(json, "codes"),
          headers = parseHexList(json, "headers"),
        ),
      )
    } catch (throwable: Throwable) {
      Err(
        ErrorResponse(
          ExecutionWitnessError.PARSE_ERROR,
          throwable.message ?: "failed to parse execution witness",
        ),
      )
    }
  }

  private fun parseHexList(json: JsonObject, field: String): List<ByteArray> {
    val array = json.getValue(field) as? JsonArray
      ?: throw IllegalArgumentException("missing or invalid field: $field")
    return array.map { element ->
      (element as String).decodeHex()
    }
  }
}

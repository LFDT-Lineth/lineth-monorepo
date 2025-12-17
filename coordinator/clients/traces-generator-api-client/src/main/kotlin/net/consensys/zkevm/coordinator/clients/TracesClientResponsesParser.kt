package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.runCatching
import io.vertx.core.json.JsonObject
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.traces.TracesCounters
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.TracesCountersV4
import net.consensys.linea.traces.TracingModuleV2
import net.consensys.linea.traces.TracingModuleV4
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

object TracesClientResponsesParser {
  private val log: Logger = LogManager.getLogger(this::class.java)

  internal fun mapErrorResponse(jsonRpcErrorResponse: JsonRpcErrorResponse): ErrorResponse<TracesServiceErrorType> {
    val errorType: TracesServiceErrorType =
      runCatching {
        TracesServiceErrorType.valueOf(jsonRpcErrorResponse.error.data.toString().substringBefore(':'))
      }.getOrElse { TracesServiceErrorType.UNKNOWN_ERROR }

    return ErrorResponse(errorType, jsonRpcErrorResponse.error.message)
  }

  internal fun parseTracesCounterResponseV2(jsonRpcResponse: JsonRpcSuccessResponse): GetTracesCountersResponse =
    parseTracesCounterResponse(
      jsonRpcResponse,
      ::parseTracesCountersV2,
    )

  internal fun parseTracesCounterResponseV4(jsonRpcResponse: JsonRpcSuccessResponse): GetTracesCountersResponse =
    parseTracesCounterResponse(
      jsonRpcResponse,
      ::parseTracesCountersV4,
    )

  internal fun parseTracesCounterResponse(
    jsonRpcResponse: JsonRpcSuccessResponse,
    parserFn: (JsonObject) -> TracesCounters,
  ): GetTracesCountersResponse {
    val result = jsonRpcResponse.result as JsonObject

    return GetTracesCountersResponse(
      result.getJsonObject("tracesCounters").let(parserFn),
      result.getString("tracesEngineVersion"),
    )
  }

  internal fun parseTracesCountersV2(tracesCounters: JsonObject): TracesCountersV2 =
    parseTracesCounters(tracesCounters, TracingModuleV2::class.java, ::TracesCountersV2) as TracesCountersV2

  internal fun parseTracesCountersV4(tracesCounters: JsonObject): TracesCountersV4 =
    parseTracesCounters(tracesCounters, TracingModuleV4::class.java, ::TracesCountersV4) as TracesCountersV4

  private fun <T : Enum<T>> parseTracesCounters(
    tracesCounters: JsonObject,
    moduleEnum: Class<T>,
    constructor: (Map<T, UInt>) -> Any,
  ): Any {
    val expectedModules = moduleEnum.enumConstants.map { it.name }.toSet()
    val evmModulesInResponse = tracesCounters.map.keys.toSet()
    val modulesMissing = expectedModules - evmModulesInResponse
    val unExpectedModules = evmModulesInResponse - expectedModules
    val error =
      if (modulesMissing.isNotEmpty()) {
        "Traces counters response is missing modules: ${modulesMissing.joinToString(",")}"
      } else if (unExpectedModules.isNotEmpty()) {
        "Traces counters has unsupported modules: ${unExpectedModules.joinToString(",")}"
      } else {
        null
      }
    if (error != null) {
      log.error(error)
      throw IllegalStateException(error)
    }

    val traces =
      moduleEnum.enumConstants.associateWith { traceModule ->
        val counterValue = tracesCounters.getString(traceModule.name)
        kotlin.runCatching { counterValue.toUInt() }
          .onFailure {
            log.error(
              "Failed to parse Evm module ${traceModule.name}='$counterValue' to UInt. errorMessage={}",
              it.message,
              it,
            )
          }
          .getOrThrow()
      }
    return constructor(traces)
  }

  internal fun parseConflatedTracesToFileResponse(jsonRpcResponse: JsonRpcSuccessResponse): GenerateTracesResponse {
    val result = jsonRpcResponse.result as JsonObject
    return GenerateTracesResponse(
      result.getString("conflatedTracesFileName"),
      result.getString("tracesEngineVersion"),
    )
  }
}

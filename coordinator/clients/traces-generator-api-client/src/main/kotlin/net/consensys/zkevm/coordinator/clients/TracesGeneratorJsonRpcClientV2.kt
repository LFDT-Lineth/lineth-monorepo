package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.mapEither
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.kotlin.core.json.get
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.JsonRpcRequestRetryer
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.isSuccess
import net.consensys.linea.traces.TracesCounters
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.TracesCountersV4
import net.consensys.linea.traces.TracesCountersV5
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger

class TracesGeneratorJsonRpcClientV2(
  private val rpcClient: JsonRpcClient,
  private val config: Config,
) :
  TracesCountersClientV2, TracesConflationClientV2 {
  constructor(
    vertx: Vertx,
    rpcClient: JsonRpcClient,
    config: Config,
    retryConfig: RequestRetryConfig,
    log: Logger = LogManager.getLogger(TracesGeneratorJsonRpcClientV2::class.java),
  ) : this(
    JsonRpcRequestRetryer(
      vertx,
      rpcClient,
      config =
      JsonRpcRequestRetryer.Config(
        methodsToRetry = retryableMethods,
        requestRetry = retryConfig,
      ),
      log = log,
    ),
    config,
  )

  data class Config(
    val expectedTracesApiVersion: String,
    val ignoreTracesGeneratorErrors: Boolean = false,
    val fallBackTracesCounters: TracesCounters,
  )

  private var requestBuilder = RequestBuilder(config.expectedTracesApiVersion)

  override fun getTracesCounters(
    blockNumber: ULong,
  ): SafeFuture<Result<GetTracesCountersResponse, ErrorResponse<TracesServiceErrorType>>> {
    val jsonRequest = requestBuilder.buildGetTracesCountersV2Request(blockNumber)

    return executeWithFallback(
      jsonRequest,
      when (config.fallBackTracesCounters) {
        is TracesCountersV2 -> TracesClientResponsesParser::parseTracesCounterResponseV2
        is TracesCountersV4 -> TracesClientResponsesParser::parseTracesCounterResponseV4
        is TracesCountersV5 -> TracesClientResponsesParser::parseTracesCounterResponseV5
        else -> throw IllegalStateException("Unsupported TracesCounters version")
      },
    ) { createFallbackTracesCountersResponse() }
  }

  override fun generateConflatedTracesToFile(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
  ): SafeFuture<Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>> {
    val jsonRequest =
      requestBuilder.buildGenerateConflatedTracesToFileV2Request(
        startBlockNumber,
        endBlockNumber,
      )
    return executeWithFallback(
      jsonRequest,
      TracesClientResponsesParser::parseConflatedTracesToFileResponse,
    ) { createFallbackConflatedTracesResponse(startBlockNumber, endBlockNumber) }
  }

  private fun createFallbackTracesCountersResponse(): GetTracesCountersResponse {
    return GetTracesCountersResponse(
      tracesCounters = config.fallBackTracesCounters,
      tracesEngineVersion = config.expectedTracesApiVersion,
    )
  }

  private fun createFallbackConflatedTracesResponse(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
  ): GenerateTracesResponse {
    val defaultFileName = "$startBlockNumber-$endBlockNumber.fake-empty.conflated.${config.expectedTracesApiVersion}.lt"
    return GenerateTracesResponse(
      tracesFileName = defaultFileName,
      tracesEngineVersion = config.expectedTracesApiVersion,
    )
  }

  private fun <T> executeWithFallback(
    jsonRequest: JsonRpcRequest,
    responseParser: (JsonRpcSuccessResponse) -> T,
    fallbackResponseProvider: () -> T,
  ): SafeFuture<Result<T, ErrorResponse<TracesServiceErrorType>>> {
    return try {
      rpcClient.makeRequest(jsonRequest).toSafeFuture()
        .thenApply { responseResult ->
          val result =
            responseResult.mapEither(
              responseParser,
              TracesClientResponsesParser::mapErrorResponse,
            )
          if (config.ignoreTracesGeneratorErrors && !result.isSuccess()) {
            Ok(fallbackResponseProvider())
          } else {
            result
          }
        }
        .exceptionally { throwable ->
          if (config.ignoreTracesGeneratorErrors) {
            Ok(fallbackResponseProvider())
          } else {
            throw throwable
          }
        }
    } catch (th: Throwable) {
      if (config.ignoreTracesGeneratorErrors) {
        SafeFuture.completedFuture(Ok(fallbackResponseProvider()))
      } else {
        throw th
      }
    }
  }

  internal class RequestBuilder(
    private val expectedTracesEngineVersion: String,
    private val id: AtomicInteger = AtomicInteger(0),
  ) {
    fun buildGetTracesCountersV2Request(blockNumber: ULong): JsonRpcRequest {
      return JsonRpcRequestListParams(
        "2.0",
        id.incrementAndGet(),
        "linea_getBlockTracesCountersV2",
        listOf(
          JsonObject.of(
            "blockNumber",
            blockNumber,
            "expectedTracesEngineVersion",
            expectedTracesEngineVersion,
          ),
        ),
      )
    }

    fun buildGenerateConflatedTracesToFileV2Request(startBlockNumber: ULong, endBlockNumber: ULong): JsonRpcRequest {
      return JsonRpcRequestListParams(
        "2.0",
        id.incrementAndGet(),
        "linea_generateConflatedTracesToFileV2",
        listOf(
          JsonObject.of(
            "startBlockNumber",
            startBlockNumber,
            "endBlockNumber",
            endBlockNumber,
            "expectedTracesEngineVersion",
            expectedTracesEngineVersion,
          ),
        ),
      )
    }
  }

  companion object {
    internal val retryableMethods = setOf("linea_getBlockTracesCountersV2", "linea_generateConflatedTracesToFileV2")

    @Suppress("UNCHECKED_CAST")
    val requestPriorityComparator =
      Comparator<JsonRpcRequest> { o1, o2 ->
        // linea_generateConflatedTracesToFileV2 is always fired after linea_getBlockTracesCountersV2
        // has successfully completed, so we should prioritize it.
        when {
          o1.method == "linea_generateConflatedTracesToFileV2" && o2.method == "linea_getBlockTracesCountersV2" -> -1
          o1.method == "linea_getBlockTracesCountersV2" && o2.method == "linea_generateConflatedTracesToFileV2" -> 1
          o1.method == "linea_getBlockTracesCountersV2" && o2.method == "linea_getBlockTracesCountersV2" -> {
            val bn1 = (o1.params as List<JsonObject>).first().get<ULong>("blockNumber")
            val bn2 = (o2.params as List<JsonObject>).first().get<ULong>("blockNumber")
            bn1.compareTo(bn2)
          }

          o1.method == "linea_generateConflatedTracesToFileV2" &&
            o2.method == "linea_generateConflatedTracesToFileV2" -> {
            val bn1 = (o1.params as List<JsonObject>).first().get<ULong>("startBlockNumber")
            val bn2 = (o2.params as List<JsonObject>).first().get<ULong>("startBlockNumber")
            bn1.compareTo(bn2)
          }

          else -> 0
        }
      }
  }
}

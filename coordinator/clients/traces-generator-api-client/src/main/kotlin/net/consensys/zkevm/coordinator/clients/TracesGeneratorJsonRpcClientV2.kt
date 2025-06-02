package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Result
import com.github.michaelbull.result.mapEither
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.JsonRpcRequestRetryer
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
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
      config = JsonRpcRequestRetryer.Config(
        methodsToRetry = retryableMethods,
        requestRetry = retryConfig,
      ),
      log = log,
    ),
    config,
  )

  data class Config(
    val expectedTracesApiVersion: String,
  )

  private var id = AtomicInteger(0)

  override fun getTracesCounters(
    blockNumber: ULong,
  ): SafeFuture<Result<GetTracesCountersResponse, ErrorResponse<TracesServiceErrorType>>> {
    val jsonRequest =
      JsonRpcRequestListParams(
        "2.0",
        id.incrementAndGet(),
        "linea_getBlockTracesCountersV2",
        listOf(
          JsonObject.of(
            "blockNumber",
            blockNumber,
            "expectedTracesEngineVersion",
            config.expectedTracesApiVersion,
          ),
        ),
      )

    return rpcClient.makeRequest(jsonRequest).toSafeFuture()
      .thenApply { responseResult ->
        responseResult.mapEither(
          TracesClientResponsesParser::parseTracesCounterResponseV2,
          TracesClientResponsesParser::mapErrorResponseV2,
        )
      }
  }

  override fun generateConflatedTracesToFile(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
  ): SafeFuture<Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>> {
    val jsonRequest =
      JsonRpcRequestListParams(
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
            config.expectedTracesApiVersion,
          ),
        ),
      )

    return rpcClient.makeRequest(jsonRequest).toSafeFuture()
      .thenApply { responseResult ->
        responseResult.mapEither(
          TracesClientResponsesParser::parseConflatedTracesToFileResponse,
          TracesClientResponsesParser::mapErrorResponseV2,
        )
      }
  }

  companion object {
    internal val retryableMethods = setOf("linea_getBlockTracesCountersV2", "linea_generateConflatedTracesToFileV2")
  }
}

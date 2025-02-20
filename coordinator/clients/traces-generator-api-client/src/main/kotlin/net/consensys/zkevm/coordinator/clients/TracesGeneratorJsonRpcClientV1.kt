package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Result
import com.github.michaelbull.result.mapEither
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import linea.domain.BlockNumberAndHash
import linea.kotlin.encodeHex
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequestMapParams
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.JsonRpcRequestRetryer
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger

class TracesGeneratorJsonRpcClientV1(
  private val rpcClient: JsonRpcClient,
  private val config: Config
) :
  TracesCountersClientV1, TracesConflationClientV1 {
  constructor(
    vertx: Vertx,
    rpcClient: JsonRpcClient,
    config: Config,
    retryConfig: RequestRetryConfig,
    log: Logger = LogManager.getLogger(TracesGeneratorJsonRpcClientV1::class.java)
  ) : this(
    JsonRpcRequestRetryer(
      vertx,
      rpcClient,
      config = JsonRpcRequestRetryer.Config(
        methodsToRetry = retryableMethods,
        requestRetry = retryConfig
      ),
      log = log
    ),
    config
  )

  data class Config(
    val rawExecutionTracesVersion: String,
    val expectedTracesApiVersion: String
  )

  private var id = AtomicInteger(0)

  override fun rollupGetTracesCounters(
    block: BlockNumberAndHash
  ): SafeFuture<Result<GetTracesCountersResponse, ErrorResponse<TracesServiceErrorType>>> {
    val jsonRequest =
      JsonRpcRequestMapParams(
        "2.0",
        id.incrementAndGet(),
        "rollup_getBlockTracesCountersV1",
        mapOf(
          "block" to mapOf(
            "blockNumber" to block.number.toString(),
            "blockHash" to block.hash.encodeHex()
          ),
          "rawExecutionTracesVersion" to config.rawExecutionTracesVersion,
          "expectedTracesApiVersion" to config.expectedTracesApiVersion
        )
      )

    return rpcClient.makeRequest(jsonRequest).toSafeFuture()
      .thenApply { responseResult ->
        responseResult.mapEither(
          TracesClientResponsesParser::parseTracesCounterResponseV1,
          TracesClientResponsesParser::mapErrorResponseV1
        )
      }
  }

  override fun rollupGenerateConflatedTracesToFile(
    blocks: List<BlockNumberAndHash>
  ): SafeFuture<Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>> {
    // TODO: validate list of blocks
    // 1 - does not have repeated/duplicated pairs
    // 2 - blocks numbers are consecutive

    val jsonRequest =
      JsonRpcRequestMapParams(
        "2.0",
        id.incrementAndGet(),
        "rollup_generateConflatedTracesToFileV1",
        mapOf(
          "blocks" to blocks.map { block ->
            JsonObject.of(
              "blockNumber",
              block.number.toString(),
              "blockHash",
              block.hash.encodeHex()
            )
          },
          "rawExecutionTracesVersion" to config.rawExecutionTracesVersion,
          "expectedTracesApiVersion" to config.expectedTracesApiVersion
        )
      )

    return rpcClient.makeRequest(jsonRequest).toSafeFuture()
      .thenApply { responseResult ->
        responseResult.mapEither(
          TracesClientResponsesParser::parseConflatedTracesToFileResponse,
          TracesClientResponsesParser::mapErrorResponseV1
        )
      }
  }

  companion object {
    internal val retryableMethods = setOf("rollup_getBlockTracesCountersV1", "rollup_generateConflatedTracesToFileV1")
  }
}

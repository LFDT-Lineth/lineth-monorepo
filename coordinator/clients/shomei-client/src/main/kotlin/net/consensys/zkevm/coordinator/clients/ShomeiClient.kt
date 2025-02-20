package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Result
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.mapEither
import com.github.michaelbull.result.runCatching
import io.vertx.core.Vertx
import linea.domain.BlockNumberAndHash
import linea.kotlin.encodeHex
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.JsonRpcRequestRetryer
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger

class ShomeiClient(
  private val rpcClient: JsonRpcClient
) : RollupForkChoiceUpdatedClient {
  constructor(
    vertx: Vertx,
    rpcClient: JsonRpcClient,
    retryConfig: RequestRetryConfig,
    log: Logger = LogManager.getLogger(ShomeiClient::class.java)
  ) : this(
    JsonRpcRequestRetryer(
      vertx,
      rpcClient,
      config = JsonRpcRequestRetryer.Config(
        methodsToRetry = retryableMethods,
        requestRetry = retryConfig
      ),
      log = log
    )
  )

  private var id = AtomicInteger(0)

  override fun rollupForkChoiceUpdated(finalizedBlockNumberAndHash: BlockNumberAndHash):
    SafeFuture<Result<RollupForkChoiceUpdatedResponse, ErrorResponse<RollupForkChoiceUpdatedError>>> {
    val jsonRequest =
      JsonRpcRequestListParams(
        "2.0",
        id.incrementAndGet(),
        "rollup_forkChoiceUpdated",
        listOf(
          mapOf(
            "finalizedBlockNumber" to finalizedBlockNumberAndHash.number.toString(),
            "finalizedBlockHash" to finalizedBlockNumberAndHash.hash.encodeHex()
          )
        )
      )
    return rpcClient.makeRequest(jsonRequest).toSafeFuture()
      .thenApply { responseResult ->
        responseResult.mapEither(ShomeiClient::mapRollupForkChoiceUpdatedResponse, ShomeiClient::mapErrorResponse)
      }
  }

  companion object {
    internal val retryableMethods = setOf("rollup_forkChoiceUpdated")
    fun mapErrorResponse(jsonRpcErrorResponse: JsonRpcErrorResponse):
      ErrorResponse<RollupForkChoiceUpdatedError> {
      val errorType: RollupForkChoiceUpdatedError = runCatching {
        RollupForkChoiceUpdatedError.valueOf(jsonRpcErrorResponse.error.message.substringBefore(':'))
      }.getOrElse { RollupForkChoiceUpdatedError.UNKNOWN }

      return ErrorResponse(errorType, jsonRpcErrorResponse.error.message)
    }

    fun mapRollupForkChoiceUpdatedResponse(jsonRpcResponse: JsonRpcSuccessResponse):
      RollupForkChoiceUpdatedResponse {
      return RollupForkChoiceUpdatedResponse(result = jsonRpcResponse.result.toString())
    }
  }
}

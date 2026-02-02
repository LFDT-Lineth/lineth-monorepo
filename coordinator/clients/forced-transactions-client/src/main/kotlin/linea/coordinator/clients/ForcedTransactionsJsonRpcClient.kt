package linea.coordinator.clients

import com.github.michaelbull.result.getOrThrow
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import linea.forcedtx.ForcedTransactionInclusionStatus
import linea.forcedtx.ForcedTransactionRequest
import linea.forcedtx.ForcedTransactionResponse
import linea.forcedtx.ForcedTransactionsClient
import linea.kotlin.encodeHex
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.JsonRpcRequestRetryer
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger

class ForcedTransactionsJsonRpcClient(
  private val rpcClient: JsonRpcClient,
) : ForcedTransactionsClient {

  constructor(
    vertx: Vertx,
    rpcClient: JsonRpcClient,
    retryConfig: RequestRetryConfig,
    log: Logger = LogManager.getLogger(ForcedTransactionsJsonRpcClient::class.java),
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
  )

  private var id = AtomicInteger(0)

  override fun lineaSendForcedRawTransaction(
    transactions: List<ForcedTransactionRequest>,
  ): SafeFuture<List<ForcedTransactionResponse>> {
    val params = transactions.map { tx ->
      JsonObject.of(
        "forcedTransactionNumber",
        tx.ftxNumber.toLong(),
        "transaction",
        tx.ftxRlp.encodeHex(),
        "deadlineBlockNumber",
        tx.deadlineBlockNumber.toString(),
      )
    }

    val jsonRequest = JsonRpcRequestListParams(
      "2.0",
      id.incrementAndGet(),
      "linea_sendForcedRawTransaction",
      listOf(params),
    )

    return rpcClient
      .makeRequest(jsonRequest, ForcedTransactionsResponseParser::parseSendForcedRawTransactionResponse)
      .toSafeFuture()
      .thenApply { responseResult ->
        val response = responseResult.getOrThrow { error ->
          RuntimeException(
            "JSON-RPC error: code=${error.error.code}, message=${error.error.message}",
          )
        }
        @Suppress("UNCHECKED_CAST")
        response.result as List<ForcedTransactionResponse>
      }
  }

  override fun lineaFindForcedTransactionStatus(
    ftxNumber: ULong,
  ): SafeFuture<ForcedTransactionInclusionStatus?> {
    val jsonRequest = JsonRpcRequestListParams(
      "2.0",
      id.incrementAndGet(),
      "linea_getForcedTransactionInclusionStatus",
      listOf(ftxNumber.toLong()),
    )

    return rpcClient
      .makeRequest(jsonRequest, ForcedTransactionsResponseParser::parseForcedTransactionInclusionStatus)
      .toSafeFuture()
      .thenApply { responseResult ->
        val response = responseResult.getOrThrow { error ->
          RuntimeException(
            "JSON-RPC error: code=${error.error.code}, message=${error.error.message}",
          )
        }
        response.result as ForcedTransactionInclusionStatus?
      }
  }

  companion object {
    internal val retryableMethods = setOf(
      "linea_sendForcedRawTransaction",
      "linea_getForcedTransactionInclusionStatus",
    )
  }
}

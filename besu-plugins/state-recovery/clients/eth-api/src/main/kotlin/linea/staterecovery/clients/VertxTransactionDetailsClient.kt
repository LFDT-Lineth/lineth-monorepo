package linea.staterecovery.clients

import com.fasterxml.jackson.databind.JsonNode
import com.github.michaelbull.result.Err
import linea.kotlin.decodeHex
import linea.staterecovery.TransactionDetailsClient
import net.consensys.linea.jsonrpc.client.JsonRpcClientFactory
import net.consensys.linea.jsonrpc.client.JsonRpcV2Client
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI

class VertxTransactionDetailsClient internal constructor(
  private val jsonRpcClient: JsonRpcV2Client
) : TransactionDetailsClient {

  companion object {
    fun create(
      jsonRpcClientFactory: JsonRpcClientFactory,
      endpoint: URI,
      retryConfig: RequestRetryConfig,
      logger: Logger = LogManager.getLogger(TransactionDetailsClient::class.java)
    ): VertxTransactionDetailsClient {
      return VertxTransactionDetailsClient(
        jsonRpcClientFactory.createJsonRpcV2Client(
          endpoints = listOf(endpoint),
          retryConfig = retryConfig,
          log = logger
        )
      )
    }
  }

  override fun getBlobVersionedHashesByTransactionHash(transactionHash: ByteArray): SafeFuture<List<ByteArray>> {
    return jsonRpcClient.makeRequest(
      "eth_getTransactionByHash",
      listOf(transactionHash),
      shallRetryRequestPredicate = { it is Err },
      resultMapper = {
        it as JsonNode
        it.get("blobVersionedHashes")
          ?.toList()
          ?.map { it.asText().decodeHex() }
          ?: emptyList()
      }
    )
  }
}

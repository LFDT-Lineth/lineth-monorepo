package linea.staterecover.clients.blobscan

import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.ext.web.client.WebClient
import io.vertx.ext.web.client.WebClientOptions
import linea.staterecover.BlobFetcher
import net.consensys.decodeHex
import net.consensys.encodeHex
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.vertx.setDefaultsFrom
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI

class BlobScanClient(
  private val restClient: RestClient<JsonObject>,
  private val log: Logger = LogManager.getLogger(BlobScanClient::class.java)
) : BlobFetcher {
  fun getBlobById(id: String): SafeFuture<ByteArray> {
    return restClient
      .get("/blobs/$id")
      .thenApply { response ->
        if (response.statusCode == 200) {
          response.body!!.getString("data").decodeHex()
        } else {
          throw RuntimeException(
            "error fetching blobId=$id " +
              "errorMessage=${response.body?.getString("message") ?: ""}"
          )
        }
      }
  }

  override fun fetchBlobsByHash(blobVersionedHashes: List<ByteArray>): SafeFuture<List<ByteArray>> {
    return SafeFuture.collectAll(blobVersionedHashes.map { hash -> getBlobById(hash.encodeHex()) }.stream())
  }

  companion object {
    fun create(
      vertx: Vertx,
      endpoint: URI,
      requestRetryConfig: RequestRetryConfig,
      logger: Logger = LogManager.getLogger(BlobScanClient::class.java),
      responseLogMaxSize: UInt? = 1000u
    ): BlobScanClient {
      val restClient = VertxRestClient(
        vertx = vertx,
        webClient = WebClient.create(vertx, WebClientOptions().setDefaultsFrom(endpoint)),
        responseParser = { it.toJsonObject() },
        retryableErrorCodes = setOf(429, 503, 504),
        requestRetryConfig = requestRetryConfig,
        log = logger,
        responseLogMaxSize = responseLogMaxSize
      )
      return BlobScanClient(restClient)
    }
  }
}

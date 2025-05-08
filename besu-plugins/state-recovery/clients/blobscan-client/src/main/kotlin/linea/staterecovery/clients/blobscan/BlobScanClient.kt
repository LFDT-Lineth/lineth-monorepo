package linea.staterecovery.clients.blobscan

import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.ext.web.client.WebClient
import io.vertx.ext.web.client.WebClientOptions
import linea.domain.RetryConfig
import linea.http.vertx.VertxHttpRequestSenderFactory
import linea.http.vertx.VertxRestLoggingFormatter
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.staterecovery.BlobFetcher
import net.consensys.linea.vertx.setDefaultsFrom
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI
import kotlin.time.Duration

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
      requestRetryConfig: RetryConfig,
      rateLimitBackoffDelay: Duration? = null,
      logger: Logger = LogManager.getLogger(BlobScanClient::class.java),
      responseLogMaxSize: UInt? = 1000u
    ): BlobScanClient {
      val logFormatter = VertxRestLoggingFormatter(responseLogMaxSize = responseLogMaxSize)

      val requestSender = VertxHttpRequestSenderFactory
        .create(
          vertx = vertx,
          requestRetryConfig = requestRetryConfig,
          rateLimitBackoffDelay = rateLimitBackoffDelay,
          retryableErrorCodes = setOf(429, 503, 504),
          logger = logger,
          requestResponseLogLevel = Level.DEBUG,
          failuresLogLevel = Level.DEBUG,
          logFormatter = logFormatter
        )
      val restClient = VertxRestClient(
        webClient = WebClient.create(vertx, WebClientOptions().setDefaultsFrom(endpoint)),
        responseParser = { it.toJsonObject() },
        requestSender = requestSender
      )
      return BlobScanClient(
        restClient = restClient,
        log = logger
      )
    }
  }
}

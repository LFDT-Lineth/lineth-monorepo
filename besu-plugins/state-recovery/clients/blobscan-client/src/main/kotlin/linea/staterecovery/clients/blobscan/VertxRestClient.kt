package linea.staterecovery.clients.blobscan

import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.WebClient
import linea.http.vertx.VertxHttpRequestSender
import tech.pegasys.teku.infrastructure.async.SafeFuture

// TODO: move to a common module
data class RestResponse<T>(
  val statusCode: Int,
  val body: T?
)

interface RestClient<Response> {
  fun get(path: String): SafeFuture<RestResponse<Response>>
  // add remaining verbs as we need them
}

class VertxRestClient<Response>(
  private val webClient: WebClient,
  private val responseParser: (Buffer) -> Response,
  private val requestSender: VertxHttpRequestSender,
  private val requestHeaders: Map<String, String> = mapOf("Accept" to "application/json")
) : RestClient<Response> {
  override fun get(path: String): SafeFuture<RestResponse<Response>> {
    return requestSender.makeRequest(
      webClient
        .get(path)
        .apply { requestHeaders.forEach(::putHeader) }
    )
      .thenApply { response ->
        val parsedResponse = response.body()?.let(responseParser)
        RestResponse(response.statusCode(), parsedResponse)
      }
  }
}

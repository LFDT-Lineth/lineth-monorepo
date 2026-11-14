package linea.jsonrpc

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.DeploymentOptions
import io.vertx.core.Future
import io.vertx.core.MultiMap
import io.vertx.core.Promise
import io.vertx.core.Vertx
import io.vertx.core.http.HttpServerResponse
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User
import io.vertx.ext.web.RoutingContext
import lineth.vertx.vertxTestOptions
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import net.consensys.linea.jsonrpc.JsonRpcMessageProcessor
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.jsonrpc.httpserver.HttpJsonRpcServer
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI
import java.net.URL
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

open class TestingJsonRpcServer(
  port: Int = 0,
  val apiPath: String = "/",
  val recordRequestsResponses: Boolean = false,
  val serverName: String = "TestingJsonRpcServer",
  loggerName: String = serverName,
  val vertx: Vertx = Vertx.vertx(vertxTestOptions),
  val responseObjectMapper: ObjectMapper = jacksonObjectMapper(),
  responsesArtificialDelay: Duration? = null,
) {
  val log: Logger = LogManager.getLogger(loggerName)
  private var httpServer: HttpJsonRpcServer = createHttpServer(port)
  val boundPort: Int
    get() = httpServer.boundPort
  private var verticleId: String? = null
  private val handlers: MutableMap<String, (JsonRpcRequest) -> Any?> = ConcurrentHashMap()
  private val methodResponseDelays: MutableMap<String, Duration> = ConcurrentHashMap()
  private val methodResponseChunkIntervals: MutableMap<String, Duration> = ConcurrentHashMap()
  private val requests: MutableList<
    Pair<JsonRpcRequest, Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>>,
    > = CopyOnWriteArrayList()
  private val requestsHeaders: MutableList<MultiMap> = CopyOnWriteArrayList()
  val httpEndpoint: URL
    get() = URI.create("http://127.0.0.1:$boundPort/$apiPath").toURL()

  var responsesArtificialDelay: Duration? = responsesArtificialDelay
    set(value) {
      require(value == null || value > 0.milliseconds) { "artificialDelay=$value must be greater than 0ms" }
      field = value
    }

  private val messageProcessor =
    JsonRpcMessageProcessor(
      requestsHandler = this::handleRequest,
      metricsFacade = MicrometerMetricsFacade(registry = SimpleMeterRegistry()),
      log = log,
      responseResultObjectMapper = responseObjectMapper,
    )

  private fun createHttpServer(port: Int?): HttpJsonRpcServer {
    return HttpJsonRpcServer(
      port = port?.toUInt() ?: 0u,
      path = apiPath,
      requestHandler = this::handleHttpRequest,
      serverName = serverName,
    )
  }

  private fun handleHttpRequest(context: RoutingContext) {
    val request = context.request()
    val response = context.response()
    request.body()
      .compose { body ->
        val requestJson = body.toString()
        if (recordRequestsResponses) {
          requestsHeaders.add(MultiMap.caseInsensitiveMultiMap().addAll(request.headers()))
        }
        messageProcessor(context.user(), requestJson).map { responseJson -> requestJson to responseJson }
      }
      .onSuccess { (requestJson, responseJson) ->
        respond(response, responseJson, responseChunkInterval(requestJson))
      }
      .onFailure { th ->
        log.error("Error handling request", th)
        if (!response.closed()) response.setStatusCode(500).end("Internal error")
      }
  }

  private fun responseChunkInterval(requestJson: String): Duration? =
    runCatching { JsonObject(requestJson).getString("method") }
      .getOrNull()
      ?.let(methodResponseChunkIntervals::get)

  private fun respond(response: HttpServerResponse, body: String, chunkInterval: Duration?) {
    if (response.closed()) return
    response.putHeader("Content-Type", "application/json")
    if (chunkInterval == null) {
      response.end(body)
    } else {
      response.setChunked(true)
      writeInChunks(response, body, 0, chunkInterval)
    }
  }

  private fun writeInChunks(response: HttpServerResponse, body: String, index: Int, interval: Duration) {
    if (response.closed()) return
    if (index == body.length) {
      response.end()
      return
    }
    response.write(body[index].toString())
    vertx.setTimer(interval.inWholeMilliseconds.coerceAtLeast(1)) { writeInChunks(response, body, index + 1, interval) }
  }

  init {
    vertx
      .deployVerticle(httpServer, DeploymentOptions().setInstances(1))
      .onSuccess { verticleId: String -> this.verticleId = verticleId }
      .get()
  }

  fun stopHttpServer(): Future<Unit> {
    return vertx.undeploy(verticleId).map { }
  }

  fun resumeHttpServer(): Future<Unit> {
    // reuse the same port
    httpServer = createHttpServer(boundPort)
    return vertx
      .deployVerticle(httpServer, DeploymentOptions().setInstances(1))
      .onSuccess { verticleId: String ->
        log.info("Http server resumed at port {}", httpServer.boundPort)
        this.verticleId = verticleId
      }
      .onFailure { th ->
        log.error("Error resuming http server", th)
      }
      .map { }
  }

  @Suppress("UNUSED_PARAMETER")
  private fun handleRequest(
    user: User?,
    jsonRpcRequest: JsonRpcRequest,
    requestJson: JsonObject,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    // need this otherwise kotlin compiler/IDE struggle to infer the type
    val result: Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> = (
      handlers[jsonRpcRequest.method]
        ?.let { handler ->
          try {
            val result = handler(jsonRpcRequest)
            Future.succeededFuture(
              Ok(
                JsonRpcSuccessResponse(
                  request = jsonRpcRequest,
                  result = result,
                ),
              ),
            )
          } catch (e: JsonRpcErrorResponseException) {
            Future.succeededFuture(Err(JsonRpcErrorResponse(jsonRpcRequest.id, e.asJsonRpcError())))
          } catch (e: Exception) {
            Future.succeededFuture(Err(JsonRpcErrorResponse.internalError(jsonRpcRequest.id, data = e.message)))
          }
        }
        ?: Future.succeededFuture(Err(JsonRpcErrorResponse.methodNotFound(jsonRpcRequest.id, jsonRpcRequest.method)))
      )

    return result
      .let { future ->
        (methodResponseDelays[jsonRpcRequest.method] ?: responsesArtificialDelay)?.let { future.delayed(it) } ?: future
      }
      .also {
        if (recordRequestsResponses) {
          requests.add(jsonRpcRequest to it)
        }
      }
  }

  /**
   * Handler shall return response result or throw [JsonRpcErrorResponseException] if error
   */
  fun handle(method: String, methodHandler: (jsonRpcRequest: JsonRpcRequest) -> Any?) {
    handlers[method] = methodHandler
  }

  /**
   * Delays the responses of [method], taking precedence over [responsesArtificialDelay].
   */
  fun delayResponses(method: String, delay: Duration) {
    require(delay > 0.milliseconds) { "delay=$delay must be greater than 0ms" }
    methodResponseDelays[method] = delay
  }

  /**
   * Streams the response body of [method] one character per [chunkInterval], to simulate a slow response body after
   * the headers are sent. Applies to single (non-batch) requests.
   */
  fun streamResponses(method: String, chunkInterval: Duration) {
    require(chunkInterval > 0.milliseconds) { "chunkInterval=$chunkInterval must be greater than 0ms" }
    methodResponseChunkIntervals[method] = chunkInterval
  }

  /**
   * HTTP headers of every received request, in arrival order. Recorded only when [recordRequestsResponses] is set.
   */
  fun recordedRequestsHeaders(): List<MultiMap> {
    return requestsHeaders.toList()
  }

  fun recordedRequests(): List<Pair<JsonRpcRequest, Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>>> {
    return requests.toList()
  }

  fun cleanRecordedRequests() {
    requests.clear()
    requestsHeaders.clear()
  }

  fun callCountByMethod(method: String): Int {
    return requests.count { it.first.method == method }
  }

  private fun <T> Future<T>.delayed(delay: Duration): Future<T> {
    val promise = Promise.promise<T>()
    vertx.setTimer(delay.inWholeMilliseconds) {
      this.onComplete(promise)
    }
    return promise.future()
  }

  private fun <T> SafeFuture<T>.delayed(delay: Duration): SafeFuture<T> {
    val promise = SafeFuture<T>()
    vertx.setTimer(delay.inWholeMilliseconds) {
      this.thenAccept(promise::complete).exceptionally {
        promise.completeExceptionally(it)
        null
      }
    }
    return promise
  }
}

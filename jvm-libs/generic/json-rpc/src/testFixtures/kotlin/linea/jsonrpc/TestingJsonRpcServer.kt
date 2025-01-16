package linea.jsonrpc

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.DeploymentOptions
import io.vertx.core.Future
import io.vertx.core.Promise
import io.vertx.core.Vertx
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.HttpRequestHandler
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import net.consensys.linea.jsonrpc.JsonRpcMessageProcessor
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.jsonrpc.httpserver.HttpJsonRpcServer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentHashMap
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

open class TestingJsonRpcServer(
  port: Int = 0,
  val apiPath: String = "/",
  val recordRequestsResponses: Boolean = false,
  val serverName: String = "TestingJsonRpcServer",
  loggerName: String = serverName,
  val vertx: Vertx = Vertx.vertx(),
  val responseObjectMapper: ObjectMapper = jacksonObjectMapper(),
  responsesArtificialDelay: Duration? = null
) {
  val log: Logger = LogManager.getLogger(loggerName)
  private var httpServer: HttpJsonRpcServer = createHttpServer(port)
  val boundPort: Int
    get() = httpServer.boundPort
  private var verticleId: String? = null
  private val handlers: MutableMap<String, (JsonRpcRequest) -> Any?> = ConcurrentHashMap()
  private var requests: MutableList<
    Pair<JsonRpcRequest, Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>>
    > = mutableListOf()

  var responsesArtificialDelay: Duration? = responsesArtificialDelay
    set(value) {
      require(value == null || value > 0.milliseconds) { "artificialDelay=$value must be greater than 0ms" }
      field = value
    }

  private fun createHttpServer(port: Int?): HttpJsonRpcServer {
    return HttpJsonRpcServer(
      port = port?.toUInt() ?: 0u,
      path = apiPath,
      requestHandler = HttpRequestHandler(
        JsonRpcMessageProcessor(
          requestsHandler = this::handleRequest,
          meterRegistry = SimpleMeterRegistry(),
          log = log,
          responseResultObjectMapper = responseObjectMapper
        )
      ),
      serverName = serverName
    )
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
    requestJson: JsonObject
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
                  result = result
                )
              )
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
        responsesArtificialDelay?.let { future.delayed(it) } ?: future
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
  fun handle(
    method: String,
    methodHandler: (jsonRpcRequest: JsonRpcRequest) -> Any?
  ) {
    handlers[method] = methodHandler
  }

  fun recordedRequests(): List<Pair<JsonRpcRequest, Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>>> {
    return requests.toList()
  }

  fun cleanRecordedRequests() {
    requests.clear()
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
      this.thenAccept(promise::complete).exceptionally { promise.completeExceptionally(it); null }
    }
    return promise
  }
}

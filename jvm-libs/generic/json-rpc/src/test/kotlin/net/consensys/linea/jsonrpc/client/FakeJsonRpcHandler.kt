package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.Promise
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcResponse
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.concurrent.timer
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

class FakeJsonRpcHandler(
  val requestHandled: MutableList<JsonRpcRequest> = CopyOnWriteArrayList(),
  var defaultResponseDelay: Duration = 0.milliseconds,
  val defaultResponseSupplier: (JsonRpcRequest) -> JsonRpcResponse = { req -> JsonRpcSuccessResponse(req.id, null) },
) : JsonRpcClient {
  private val responseSuppliers: MutableMap<
    JsonRpcRequest,
    Pair<
      Duration,
      (
        JsonRpcRequest,
      ) -> JsonRpcResponse,
      >,
    > = ConcurrentHashMap()

  fun onRequest(
    request: JsonRpcRequest,
    delay: Duration = 0.milliseconds,
    responseSupplier: (JsonRpcRequest) -> JsonRpcResponse,
  ) {
    responseSuppliers[request] = delay to responseSupplier
  }

  private fun buildResponse(
    request: JsonRpcRequest,
    responseSupplier: (JsonRpcRequest) -> JsonRpcResponse,
  ): Result<JsonRpcSuccessResponse, JsonRpcErrorResponse> {
    return responseSupplier(request)
      .let { response ->
        when (response) {
          is JsonRpcSuccessResponse -> Ok(response)
          is JsonRpcErrorResponse -> Err(response)
          else -> throw IllegalStateException("Invalid response type=${response.javaClass}: $response")
        }
      }
  }

  private fun getResponseSupplier(request: JsonRpcRequest): Pair<Duration, (JsonRpcRequest) -> JsonRpcResponse> {
    val delayAndSupplier = responseSuppliers[request]
    return if (delayAndSupplier != null) {
      delayAndSupplier
    } else {
      defaultResponseDelay to defaultResponseSupplier
    }
  }

  override fun makeRequest(
    request: JsonRpcRequest,
    resultMapper: (Any?) -> Any?,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    this.requestHandled.add(request)

    val (delay, responseSupplier) = getResponseSupplier(request)
    if (delay <= 0.milliseconds) {
      return Future.succeededFuture(buildResponse(request, responseSupplier))
    }
    val promise = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
    timer("rcp-client", false, delay.inWholeMilliseconds, 100L) {
      if (!promise.future().isComplete) {
        promise.complete(buildResponse(request, responseSupplier))
      }
      this.cancel()
    }

    return promise.future()
  }
}

package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.CompositeFuture
import io.vertx.core.Future
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse

class JsonRpcRequestFanOut(
  private val targets: List<JsonRpcClient>,
) : JsonRpcClient {
  init {
    require(targets.isNotEmpty()) { "Must have at least one target to fan out the requests" }
  }

  /**
   * Sends the request to all targets. If one of the target returns a failed response, will return that failed
   * response. If all targets return a successful response, will return the first successful response.
   *
   * Not ideal if handle needs to handle each response individually.
   */
  override fun makeRequest(
    request: JsonRpcRequest,
    resultMapper: (Any?) -> Any?,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    return this.fanoutRequest(request, resultMapper).map { results ->
      val errors = results.filterIsInstance<Err<JsonRpcErrorResponse>>()
      if (errors.isNotEmpty()) {
        errors.first()
      } else {
        val successes = results.filterIsInstance<Ok<JsonRpcSuccessResponse>>()
        successes.first()
      }
    }
  }

  fun fanoutRequest(
    request: JsonRpcRequest,
    resultMapper: (Any?) -> Any? = ::toPrimitiveOrVertxJson,
  ): Future<List<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>> {
    return Future
      .all(targets.map { it.makeRequest(request, resultMapper) })
      .map { it: CompositeFuture -> it.list() }
  }
}

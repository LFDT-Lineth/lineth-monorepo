package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Result
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Predicate

/**
 * JSON-RPC client that supports JSON-RPC v2.0.
 * It will automatically generate the request id and retry requests when JSON-RPC errors are received.
 * Please override default stopRetriesPredicate to customize the retry logic.
 *
 * JSON-RPC result/error.data serialization is done automatically to Jackson JsonNode or primitive types.
 */
interface JsonRpcV2Client {
  /**
   * Makes a JSON-RPC request.
   * @param method The method to call.
   * @param params The parameters to pass to the method. It can be a List<Any?>, a Map<String, *> or a Pojo.
   * @param shallRetryRequestPredicate predicate to evaluate request retrying. It defaults to never retrying.
   * @param resultMapper Mapper to apply to successful JSON-RPC result.
   *  the result is primary type (String, Number, Boolean, null) or (jackson's JsonNode or vertx JsonObject/JsonArray)
   *  The underlying type will depend on the serialization configured on the concrete implementation.
   * @return A future that
   *  - when success - resolves with mapped result
   *  - when JSON-RPC error - rejects with JsonRpcErrorException with corresponding error code, message and data
   *  - when other error - rejects with underlying exception
   */
  fun <T> makeRequest(
    method: String,
    params: Any, // List<Any?>, Map<String, Any?>, Pojo
    shallRetryRequestPredicate: Predicate<Result<T, Throwable>> = Predicate { false },
    resultMapper: (Any?) -> T,
  ): SafeFuture<T>
}

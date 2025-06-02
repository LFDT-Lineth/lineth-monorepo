package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Result
import net.consensys.linea.jsonrpc.JsonRpcRequestData
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Predicate
import java.util.function.Supplier

internal class JsonRpcV2ClientImpl(
  private val delegate: JsonRpcRequestRetryerV2,
  private val idSupplier: Supplier<Any>,
) : JsonRpcV2Client {

  override fun <T> makeRequest(
    method: String,
    params: Any,
    shallRetryRequestPredicate: Predicate<Result<T, Throwable>>,
    resultMapper: (Any?) -> T,
  ): SafeFuture<T> {
    val request = JsonRpcRequestData(jsonrpc = "2.0", id = idSupplier.get(), method, params)

    return delegate.makeRequest(
      request = request,
      shallRetryRequestPredicate = shallRetryRequestPredicate,
      resultMapper = resultMapper,
    )
  }
}

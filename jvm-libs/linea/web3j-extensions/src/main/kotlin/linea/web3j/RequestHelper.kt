package linea.web3j

import linea.error.JsonRpcErrorResponseException
import net.consensys.linea.async.toSafeFuture
import org.web3j.protocol.core.RemoteFunctionCall
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun <Resp> rejectOnJsonRpcError(
  rpcMethod: String,
  response: Resp,
): SafeFuture<Resp>
  where Resp : Response<*> {
  return if (response.hasError()) {
    SafeFuture.failedFuture(
      JsonRpcErrorResponseException(
        rpcErrorCode = response.error.code,
        rpcErrorMessage = response.error.message,
        rpcErrorData = response.error.data,
        method = rpcMethod,
      ),
    )
  } else {
    SafeFuture.completedFuture(response)
  }
}

fun <Resp, T> Request<*, Resp>.requestAsync(
  mapperFn: (Resp) -> T,
): SafeFuture<T>
  where Resp : Response<*> {
  return this.sendAsync()
    .thenCompose { response -> rejectOnJsonRpcError(this.method, response) }
    .toSafeFuture()
    .thenApply(mapperFn)
}

fun <R, T> RemoteFunctionCall<R>.requestAsync(mapperFn: (R) -> T): SafeFuture<T> {
  return sendAsync()
    .toSafeFuture()
    .thenApply(mapperFn)
}

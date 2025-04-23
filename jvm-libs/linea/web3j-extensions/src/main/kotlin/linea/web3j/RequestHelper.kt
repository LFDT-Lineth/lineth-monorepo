package linea.web3j

import net.consensys.linea.async.toSafeFuture
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun <T> handleError(
  response: Response<T>
): SafeFuture<T> {
  return if (response.hasError()) {
    SafeFuture.failedFuture(
      RuntimeException(
        "json-rpc error: code=${response.error.code} message=${response.error.message} " +
          "data=${response.error.data}"
      )
    )
  } else {
    SafeFuture.completedFuture(response.result)
  }
}

fun <Resp, RespT, T> Request<*, Resp>.requestAsync(
  mapperFn: (RespT) -> T
): SafeFuture<T>
  where Resp : Response<RespT> {
  return this.sendAsync()
    .thenCompose(::handleError)
    .toSafeFuture()
    .thenApply(mapperFn)
}

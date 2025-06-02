package build.linea.clients

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import net.consensys.linea.errors.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Marker interface for error types.
 * Allow concrete clients to extend this interface to define their own error types.
 */
interface ClientError

class ClientException(
  override val message: String,
  val errorType: ClientError?,
) :
  RuntimeException(errorType?.let { "errorType=$it $message" } ?: message)

interface Client<Request, Response> {
  fun makeRequest(request: Request): Response
}

interface ClientRequest<Response>
interface AsyncClient<ReqSupperType> {
  fun <Response> makeRequest(request: ClientRequest<Response>): SafeFuture<Response>
}

fun <T, E : ClientError> SafeFuture<Result<T, ErrorResponse<E>>>.unwrapResultMonad(): SafeFuture<T> {
  return this.thenCompose {
    when (it) {
      is Ok -> SafeFuture.completedFuture(it.value)
      is Err -> SafeFuture.failedFuture(ClientException(it.error.message, it.error.type))
    }
  }
}

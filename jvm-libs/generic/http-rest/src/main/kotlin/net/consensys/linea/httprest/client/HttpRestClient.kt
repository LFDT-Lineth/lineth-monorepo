package net.consensys.linea.httprest.client

import com.github.michaelbull.result.Result
import io.vertx.core.buffer.Buffer
import net.consensys.linea.errors.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class RestErrorType(val code: Int?, val message: String?) {
  BAD_REQUEST(400, "Bad Request"),
  UNAUTHORIZED(401, "Unauthorized"),
  FORBIDDEN(403, "Forbidden"),
  NOT_FOUND(404, "Not Found"),
  INTERNAL_SERVER_ERROR(500, "Internal server error"),
  BAD_GATEWAY(502, "Bad Gateway"),
  SERVICE_UNAVAILABLE(503, "Service Unavailable"),
  UNKNOWN(null, null),
  ;

  companion object {
    fun fromStatusCode(code: Int?): RestErrorType {
      return RestErrorType.values().first { it.code == code }
    }
  }
}

fun identityMapper(value: Any?): Any? = value

interface HttpRestClient {

  fun get(
    path: String,
    params: List<Pair<String, String>> = emptyList(),
    resultMapper: (Any?) -> Any? = ::identityMapper,
  ): SafeFuture<Result<Any?, ErrorResponse<RestErrorType>>>

  fun post(
    path: String,
    buffer: Buffer,
    resultMapper: (Any?) -> Any? = ::identityMapper,
  ): SafeFuture<Result<Any?, ErrorResponse<RestErrorType>>>
}

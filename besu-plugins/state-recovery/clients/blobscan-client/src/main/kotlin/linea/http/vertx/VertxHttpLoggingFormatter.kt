package linea.http.vertx

import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpRequest
import io.vertx.ext.web.client.HttpResponse

interface VertxHttpLoggingFormatter {
  fun toLogString(request: HttpRequest<Buffer>): String
  fun toLogString(
    request: HttpRequest<Buffer>,
    response: HttpResponse<Buffer>? = null,
    failureCause: Throwable? = null,
  ): String
}

fun HttpRequest<*>.fullUri(): String {
  val scheme = if (this.ssl()) "https" else "http"
  val path = if (this.uri().startsWith("/")) this.uri() else "/${this.uri()}"

  return String.format(
    "%s://%s:%s%s",
    scheme,
    this.host(),
    this.port(),
    path,
  )
}

class VertxRestLoggingFormatter(
  private val includeFullUri: Boolean = false,
  private val uriTransformer: (String) -> String = { it },
  private val responseLogMaxSize: UInt? = null,
) : VertxHttpLoggingFormatter {
  fun HttpRequest<*>.uriToLog(): String {
    return if (includeFullUri) {
      this.fullUri()
    } else {
      this.uri()
    }
  }

  override fun toLogString(request: HttpRequest<Buffer>): String {
    return String.format("--> %s %s", request.method(), uriTransformer.invoke(request.uriToLog()))
  }

  override fun toLogString(
    request: HttpRequest<Buffer>,
    response: HttpResponse<Buffer>?,
    failureCause: Throwable?,
  ): String {
    return if (failureCause != null) {
      String.format(
        "<-- %s %s %s",
        request.method(),
        uriTransformer.invoke(request.uriToLog()),
        failureCause.message?.let { errorMsg -> "error=$errorMsg" } ?: "",
      )
    } else {
      @Suppress("INACCESSIBLE_TYPE")
      val responseToLog: String? = response?.bodyAsString()?.let { bodyStr ->
        if (responseLogMaxSize != null) {
          bodyStr.take(responseLogMaxSize.toInt()) + "..." + "(contentLength=${response.getHeader("Content-Length")})"
        } else {
          bodyStr
        }
      }
      String.format(
        "<-- %s %s %s %s",
        request.method(),
        uriTransformer.invoke(request.uriToLog()),
        response?.statusCode() ?: "",
        responseToLog,
      )
    }
  }
}

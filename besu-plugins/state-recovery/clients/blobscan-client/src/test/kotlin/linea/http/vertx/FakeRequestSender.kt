package linea.http.vertx

import io.vertx.core.buffer.Buffer
import io.vertx.core.http.HttpVersion
import io.vertx.core.http.impl.headers.HeadersMultiMap
import io.vertx.ext.web.client.HttpRequest
import io.vertx.ext.web.client.HttpResponse
import io.vertx.ext.web.client.impl.HttpResponseImpl
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.time.Duration
import kotlin.time.TimeSource

fun httpResponse(
  statusCode: Int = 200,
  statusMessage: String = "OK",
  body: Buffer = Buffer.buffer()
): HttpResponse<Buffer> {
  return HttpResponseImpl(
    /* version = */ HttpVersion.HTTP_1_1,
    /* statusCode = */ statusCode,
    /* statusMessage = */ statusMessage,
    /* headers = */ HeadersMultiMap(),
    /* trailers = */ HeadersMultiMap(),
    /* cookies = */ emptyList<String>(),
    /* body = */ body,
    /* redirects = */ emptyList<String>()
  )
}

class FakeRequestSender(
  val requestLogger: VertxRequestLogger =
    VertxRestRequestLogger(
      responseLogMaxSize = null,
      requestResponseLogLevel = Level.DEBUG,
      log = LogManager.getLogger(FakeRequestSender::class.java)
    )
) : VertxHttpRequestSender {
  private val monotonicClock = TimeSource.Monotonic
  private var lastRequestTime = monotonicClock.markNow()
  val requestsTimesDiffs = CopyOnWriteArrayList<Duration>()

  @get:Synchronized
  @set:Synchronized
  var responseStatusCode: Int = 200

  override fun makeRequest(request: HttpRequest<Buffer>): SafeFuture<HttpResponse<Buffer>> {
    requestLogger.logRequest(request)
    requestsTimesDiffs.add(lastRequestTime.elapsedNow())

    synchronized(this) {
      lastRequestTime = monotonicClock.markNow()
    }

    val response = httpResponse(200)

    return SafeFuture.completedFuture(response)
  }
}

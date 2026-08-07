package lineth.jsonrpc

import io.vertx.core.Handler
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.RoutingContext
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class HttpRequestHandler(private val jsonRpcMessageProcessor: JsonRpcMessageHandler) :
  Handler<RoutingContext> {
  private val log: Logger = LogManager.getLogger(this.javaClass)
  override fun handle(request: RoutingContext) {
    request.request().bodyHandler { buffer: Buffer ->
      jsonRpcMessageProcessor
        .invoke(request.user(), buffer.toString())
        .onSuccess { bodyStr: String? ->
          request.response().putHeader("Content-Type", "application/json").end(bodyStr)
        }
        .onFailure { throwable: Throwable ->
          log.error("{}", throwable)
          // We should not send throwable.message in response because:
          // 1. We may leak internal information
          // 2. If message is null, Vertex does not doesn't close the connection properly
          request.response().setStatusCode(500).end("Internal error")
        }
    }
  }
}

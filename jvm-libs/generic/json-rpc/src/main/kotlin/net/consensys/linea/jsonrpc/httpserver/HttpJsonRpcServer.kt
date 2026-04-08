package net.consensys.linea.jsonrpc.httpserver

import io.vertx.core.AbstractVerticle
import io.vertx.core.Handler
import io.vertx.core.Promise
import io.vertx.core.http.HttpServer
import io.vertx.core.http.HttpServerOptions
import io.vertx.ext.web.Router
import io.vertx.ext.web.RoutingContext
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class HttpJsonRpcServer(
  private val port: UInt,
  private val path: String,
  private val requestHandler: Handler<RoutingContext>,
  val serverName: String = "",
) : AbstractVerticle() {
  private val log: Logger = LogManager.getLogger(this.javaClass)
  private lateinit var httpServer: HttpServer
  val boundPort: Int
    get() = if (this::httpServer.isInitialized) {
      httpServer.actualPort()
    } else {
      throw IllegalStateException("Http server not started")
    }

  override fun start(startPromise: Promise<Void>) {
    val options = HttpServerOptions().setPort(port.toInt()).setReusePort(true)
    log.debug("creating http server: name={} port={}", serverName, port)
    httpServer = vertx.createHttpServer(options)
    httpServer.requestHandler(buildRouter())
    httpServer.listen()
      .onSuccess { httpServer ->
        log.info(
          "http server started: name={} listening port={}",
          serverName,
          httpServer.actualPort(),
        )
        startPromise.complete()
      }.onFailure { th ->
        log.error("error creating http server: name={} errorMessage={}", serverName, th.message, th)
        startPromise.fail(th)
      }
  }

  private fun buildRouter(): Router {
    val router = Router.router(vertx)
    router.route(path).produces("application/json").handler(requestHandler)
    return router
  }

  override fun stop(endFuture: Promise<Void>) {
    httpServer
      .close()
      .onSuccess {
        super.stop(endFuture)
      }
      .onFailure { th ->
        log.error("error closing http server: name={} errorMessage={}", serverName, th.message, th)
        endFuture.fail(th)
      }
  }
}

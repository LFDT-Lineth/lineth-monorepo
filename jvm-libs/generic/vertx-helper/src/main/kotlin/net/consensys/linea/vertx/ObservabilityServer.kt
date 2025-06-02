package net.consensys.linea.vertx

import io.vertx.core.AbstractVerticle
import io.vertx.core.AsyncResult
import io.vertx.core.Handler
import io.vertx.core.Promise
import io.vertx.core.http.HttpHeaders
import io.vertx.core.http.HttpServer
import io.vertx.core.json.JsonObject
import io.vertx.ext.healthchecks.HealthCheckHandler
import io.vertx.ext.healthchecks.Status
import io.vertx.ext.web.Router
import io.vertx.ext.web.RoutingContext
import io.vertx.micrometer.PrometheusScrapingHandler
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class ObservabilityServer(private val config: Config) : AbstractVerticle() {

  data class Config(
    val applicationName: String,
    val port: Int = 0, // 0 means random port will be assigned by the underlying OS
    val livenessPath: String = "/live",
    val readinessPath: String = "/ready",
    val metricsPath: String = "/metrics",
    val healthPath: String = "/health",
    val metricsHandler: Handler<RoutingContext> = PrometheusScrapingHandler.create(),
    val readinessSupplier: (() -> Boolean) = { true },
    val healthCheckHandler: HealthCheckHandler? = null,
  )

  private var actualPort: Int? = null
  val port: Int
    get() = actualPort ?: throw IllegalStateException("Server not started")
  private val log: Logger = LogManager.getLogger(this::class.java)
  private val okReply = JsonObject().put("status", "OK").encode()
  private var started = false

  override fun start(startPromise: Promise<Void>) {
    val healthCheckHandler =
      config.healthCheckHandler
        ?: HealthCheckHandler.create(vertx).register(config.applicationName) { future ->
          future.complete(if (started) Status.OK() else Status.KO())
        }
    val router = Router.router(vertx)
    router.get(config.livenessPath).handler(::live)
    router.get(config.readinessPath).handler { rc: RoutingContext ->
      ready(rc, config.readinessSupplier)
    }
    router.get(config.metricsPath).handler(config.metricsHandler)
    router.get(config.healthPath).handler(healthCheckHandler)

    vertx.createHttpServer().requestHandler(router).listen(config.port) {
        res: AsyncResult<HttpServer> ->
      started = res.succeeded()
      if (started) {
        actualPort = res.result().actualPort()
        log.info("Monitoring Server started and listening on port {}", actualPort)
        startPromise.complete()
      } else {
        startPromise.fail(res.cause())
      }
    }
  }

  private fun ready(rc: RoutingContext, isReadySupplier: () -> Boolean) {
    if (isReadySupplier()) {
      rc.response().putHeader(HttpHeaders.CONTENT_TYPE, "application/json").end(okReply)
    } else {
      rc.response()
        .apply {
          this.setStatusCode(503)
          this.setStatusMessage("Service Unavailable")
        }
        .end("Service Unavailable")
    }
  }

  private fun live(rc: RoutingContext) {
    rc.response().putHeader(HttpHeaders.CONTENT_TYPE, "application/json").end(okReply)
  }
}

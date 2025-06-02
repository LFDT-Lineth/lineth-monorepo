package net.consensys.zkevm.coordinator.api

import io.vertx.core.Future
import io.vertx.core.Vertx
import net.consensys.linea.vertx.ObservabilityServer

class Api(
  private val configs: Config,
  private val vertx: Vertx,
) {
  data class Config(
    val observabilityPort: UInt,
  )

  private var observabilityServerId: String? = null

  fun start(): Future<*> {
    val observabilityServer =
      ObservabilityServer(ObservabilityServer.Config("coordinator", configs.observabilityPort.toInt()))
    return vertx.deployVerticle(observabilityServer)
  }

  fun stop(): Future<*> {
    return this.observabilityServerId?.let { vertx.undeploy(it) } ?: Future.succeededFuture(null)
  }
}

package net.consensys.linea.traces.app.api

import io.vertx.core.DeploymentOptions
import io.vertx.core.Future
import io.vertx.core.Vertx
import net.consensys.linea.TracesConflationServiceV1
import net.consensys.linea.TracesCountingServiceV1
import net.consensys.linea.jsonrpc.HttpRequestHandler
import net.consensys.linea.jsonrpc.JsonRpcMessageHandler
import net.consensys.linea.jsonrpc.JsonRpcMessageProcessor
import net.consensys.linea.jsonrpc.JsonRpcRequestRouter
import net.consensys.linea.jsonrpc.httpserver.HttpJsonRpcServer
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.vertx.ObservabilityServer

data class ApiConfig(
  val port: UInt,
  val observabilityPort: UInt,
  val path: String = "/",
  val numberOfVerticles: UInt
)

class Api(
  private val configs: ApiConfig,
  private val vertx: Vertx,
  private val metricsFacade: MetricsFacade,
  private val semVerValidator: TracesSemanticVersionValidator,
  private val tracesCountingService: TracesCountingServiceV1,
  private val tracesConflationService: TracesConflationServiceV1
) {
  private var jsonRpcServerId: String? = null
  private var observabilityServerId: String? = null
  fun start(): Future<*> {
    val requestHandlersV1 =
      mapOf(
        ApiMethod.ROLLUP_GET_BLOCK_TRACES_COUNTERS_V1.method to
          TracesCounterRequestHandlerV1(tracesCountingService, semVerValidator),
        ApiMethod.ROLLUP_GENERATE_CONFLATED_TRACES_TO_FILE_V1.method to
          GenerateConflatedTracesToFileRequestHandlerV1(tracesConflationService, semVerValidator),
        // Just for Debug/Dev Purposes
        ApiMethod.ROLLUP_GET_CONFLATED_TRACES_V1.method to
          GetConflatedTracesRequestHandlerV1(tracesConflationService, semVerValidator)
      )

    val messageHandler: JsonRpcMessageHandler =
      JsonRpcMessageProcessor(JsonRpcRequestRouter(requestHandlersV1), metricsFacade)

    val numberOfVerticles: Int =
      if (configs.numberOfVerticles.toInt() > 0) {
        configs.numberOfVerticles.toInt()
      } else {
        Runtime.getRuntime().availableProcessors()
      }

    val observabilityServer =
      ObservabilityServer(ObservabilityServer.Config("traces-api", configs.observabilityPort.toInt()))
    return vertx
      .deployVerticle(
        { HttpJsonRpcServer(configs.port, configs.path, HttpRequestHandler(messageHandler)) },
        DeploymentOptions().setInstances(numberOfVerticles)
      )
      .compose { verticleId: String ->
        jsonRpcServerId = verticleId
        vertx.deployVerticle(observabilityServer).onSuccess { monitorVerticleId ->
          this.observabilityServerId = monitorVerticleId
        }
      }
  }

  fun stop(): Future<*> {
    return Future.all(
      this.jsonRpcServerId?.let { vertx.undeploy(it) } ?: Future.succeededFuture(null),
      this.observabilityServerId?.let { vertx.undeploy(it) } ?: Future.succeededFuture(null)
    )
  }
}

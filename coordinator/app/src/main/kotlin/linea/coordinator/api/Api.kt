package linea.coordinator.api

import io.vertx.core.Deployable
import io.vertx.core.DeploymentOptions
import io.vertx.core.Future
import io.vertx.core.Vertx
import linea.coordinator.api.requesthandlers.ConflationCreateProverRequestHandler
import linea.coordinator.api.requesthandlers.ConflationGetJobStatusRequestHandler
import linea.coordinator.api.requesthandlers.ConflationStopJobRequestHandler
import linea.coordinator.api.requesthandlers.ConflationTargetCheckpointResumeRequestHandler
import linea.coordinator.app.conflationbacktesting.ConflationBacktestingService
import lineth.LongRunningService
import lineth.async.toSafeFuture
import lineth.jsonrpc.HttpRequestHandler
import lineth.jsonrpc.JsonRpcMessageHandler
import lineth.jsonrpc.JsonRpcMessageProcessor
import lineth.jsonrpc.JsonRpcRequestHandler
import lineth.jsonrpc.JsonRpcRequestRouter
import lineth.jsonrpc.httpserver.HttpJsonRpcServer
import lineth.metrics.MetricsFacade
import lineth.vertx.ObservabilityServer
import org.apache.logging.log4j.LogManager
import java.util.concurrent.CompletableFuture
import java.util.function.Supplier

class Api(
  private val configs: Config,
  private val vertx: Vertx,
  private val conflationBacktestingService: ConflationBacktestingService,
  private val metricsFacade: MetricsFacade,
  private val conflationCheckpointResumeLatch: () -> Boolean,
  // Extra JSON-RPC methods contributed by coordinator extensions, merged into the core router.
  private val additionalRequestHandlers: Map<String, JsonRpcRequestHandler> = emptyMap(),
) : LongRunningService {
  data class Config(
    val observabilityPort: UInt,
    val jsonRpcPort: UInt,
    val jsonRpcPath: String,
    val jsonRpcServerVerticles: Int,
  )

  private val log = LogManager.getLogger(Api::class.java)

  private var observabilityServerId: String? = null
  private var jsonRpcServerId: String? = null
  private var serverPort: Int = -1

  val bindedPort: Int
    get() = if (serverPort > 0) {
      serverPort
    } else {
      throw IllegalStateException("Http server not started")
    }

  override fun start(): CompletableFuture<Unit> {
    val requestHandlers = mapOf(
      ConflationCreateProverRequestHandler.METHOD_NAME to
        ConflationCreateProverRequestHandler(conflationBacktestingService = conflationBacktestingService),
      ConflationGetJobStatusRequestHandler.METHOD_NAME to
        ConflationGetJobStatusRequestHandler(conflationBacktestingService = conflationBacktestingService),
      ConflationStopJobRequestHandler.METHOD_NAME to
        ConflationStopJobRequestHandler(conflationBacktestingService = conflationBacktestingService),
      ConflationTargetCheckpointResumeRequestHandler.METHOD_NAME to
        ConflationTargetCheckpointResumeRequestHandler(signalResume = conflationCheckpointResumeLatch),
    ).let { coreHandlers ->
      val clashes = coreHandlers.keys.intersect(additionalRequestHandlers.keys)
      require(clashes.isEmpty()) { "Extension JSON-RPC methods clash with core methods: $clashes" }
      coreHandlers + additionalRequestHandlers
    }
    val messageHandler: JsonRpcMessageHandler =
      JsonRpcMessageProcessor(JsonRpcRequestRouter(requestHandlers), metricsFacade)

    val observabilityServer =
      ObservabilityServer(ObservabilityServer.Config("coordinator", configs.observabilityPort.toInt()))

    var httpServer: HttpJsonRpcServer? = null
    val numberOfVerticles = if (configs.jsonRpcServerVerticles > 0) {
      configs.jsonRpcServerVerticles
    } else {
      Runtime.getRuntime().availableProcessors()
    }
    return vertx
      .deployVerticle(
        Supplier<Deployable> {
          HttpJsonRpcServer(configs.jsonRpcPort, configs.jsonRpcPath, HttpRequestHandler(messageHandler))
            .also {
              httpServer = it
            }
        },
        DeploymentOptions().setInstances(numberOfVerticles),
      )
      .compose { verticleId: String ->
        jsonRpcServerId = verticleId
        serverPort = httpServer!!.boundPort
        log.info("JSON-RPC server started port={}", serverPort)
        vertx.deployVerticle(observabilityServer).onSuccess { monitorVerticleId ->
          this.observabilityServerId = monitorVerticleId
        }
      }.toSafeFuture().thenApply { }
  }

  override fun stop(): CompletableFuture<Unit> {
    return Future.all(
      this.jsonRpcServerId?.let { vertx.undeploy(it) } ?: Future.succeededFuture(Unit),
      this.observabilityServerId?.let { vertx.undeploy(it) } ?: Future.succeededFuture(Unit),
    ).toSafeFuture().thenApply {}
  }
}

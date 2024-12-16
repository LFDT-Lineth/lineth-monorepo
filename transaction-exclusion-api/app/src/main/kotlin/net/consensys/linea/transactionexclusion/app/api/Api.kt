package net.consensys.linea.transactionexclusion.app.api

import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.DeploymentOptions
import io.vertx.core.Future
import io.vertx.core.Vertx
import net.consensys.linea.jsonrpc.HttpRequestHandler
import net.consensys.linea.jsonrpc.JsonRpcMessageHandler
import net.consensys.linea.jsonrpc.JsonRpcMessageProcessor
import net.consensys.linea.jsonrpc.JsonRpcRequestRouter
import net.consensys.linea.jsonrpc.httpserver.HttpJsonRpcServer
import net.consensys.linea.transactionexclusion.TransactionExclusionServiceV1
import net.consensys.linea.vertx.ObservabilityServer

data class ApiConfig(
  val port: Int = 0,
  val observabilityPort: Int = 0,
  val numberOfVerticles: Int = 0,
  val path: String = "/"
)

class Api(
  private val configs: ApiConfig,
  private val vertx: Vertx,
  private val meterRegistry: MeterRegistry,
  private val transactionExclusionService: TransactionExclusionServiceV1
) {
  private var jsonRpcServerId: String? = null
  private var observabilityServerId: String? = null
  private var serverPort: Int = -1
  val bindedPort: Int
    get() = if (serverPort > 0) {
      serverPort
    } else {
      throw IllegalStateException("Http server not started")
    }

  fun start(): Future<*> {
    val requestHandlersV1 =
      mapOf(
        ApiMethod.LINEA_SAVE_REJECTED_TRANSACTION_V1.method to
          SaveRejectedTransactionRequestHandlerV1(
            transactionExclusionService = transactionExclusionService
          ),
        ApiMethod.LINEA_GET_TRANSACTION_EXCLUSION_STATUS_V1.method to
          GetTransactionExclusionStatusRequestHandlerV1(
            transactionExclusionService = transactionExclusionService
          )
      )

    val messageHandler: JsonRpcMessageHandler =
      JsonRpcMessageProcessor(JsonRpcRequestRouter(requestHandlersV1), meterRegistry)

    val numberOfVerticles: Int =
      if (configs.numberOfVerticles > 0) {
        configs.numberOfVerticles
      } else {
        Runtime.getRuntime().availableProcessors()
      }

    val observabilityServer =
      ObservabilityServer(
        ObservabilityServer.Config(
          "transaction-exclusion-api",
          configs.observabilityPort
        )
      )
    var httpServer: HttpJsonRpcServer? = null
    return vertx
      .deployVerticle(
        {
          HttpJsonRpcServer(configs.port.toUInt(), configs.path, HttpRequestHandler(messageHandler))
            .also {
              httpServer = it
            }
        },
        DeploymentOptions().setInstances(numberOfVerticles)
      )
      .compose { verticleId: String ->
        jsonRpcServerId = verticleId
        serverPort = httpServer!!.boundPort
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

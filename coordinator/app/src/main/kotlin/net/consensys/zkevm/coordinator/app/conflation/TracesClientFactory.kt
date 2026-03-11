package net.consensys.zkevm.coordinator.app.conflation

import linea.coordinator.config.toJsonRpcRetry
import linea.coordinator.config.v2.TracesConfig
import linea.coordinator.config.v2.TracesConfig.ClientApiConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.traces.TracesCounters
import net.consensys.zkevm.coordinator.clients.TracesGeneratorJsonRpcClientV2
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

data class TracesClients(
  val tracesCountersClient: TracesGeneratorJsonRpcClientV2,
  val tracesConflationClient: TracesGeneratorJsonRpcClientV2,
)

object TracesClientFactory {
  fun createTracesClient(
    vertx: io.vertx.core.Vertx,
    rpcClientFactory: VertxHttpJsonRpcClientFactory,
    apiConfig: ClientApiConfig,
    ignoreTracesGeneratorErrors: Boolean,
    expectedTracesApiVersion: String,
    fallBackTracesCounters: TracesCounters,
    logger: Logger,
  ): TracesGeneratorJsonRpcClientV2 {
    return TracesGeneratorJsonRpcClientV2(
      vertx = vertx,
      rpcClient =
      rpcClientFactory.createWithLoadBalancing(
        endpoints = apiConfig.endpoints.toSet(),
        maxInflightRequestsPerClient = apiConfig.requestLimitPerEndpoint,
        requestTimeout = apiConfig.requestTimeout?.inWholeMilliseconds,
        log = logger,
        requestPriorityComparator = TracesGeneratorJsonRpcClientV2.requestPriorityComparator,
      ),
      config =
      TracesGeneratorJsonRpcClientV2.Config(
        expectedTracesApiVersion = expectedTracesApiVersion,
        ignoreTracesGeneratorErrors = ignoreTracesGeneratorErrors,
        fallBackTracesCounters = fallBackTracesCounters,
      ),
      retryConfig = apiConfig.requestRetries.toJsonRpcRetry(),
      log = logger,
    )
  }

  fun createTracesClients(
    vertx: io.vertx.core.Vertx,
    rpcClientFactory: VertxHttpJsonRpcClientFactory,
    configs: TracesConfig,
    fallBackTracesCounters: TracesCounters,
    log: Logger? = null,
  ): TracesClients {
    return when {
      configs.common != null -> {
        val commonClient =
          createTracesClient(
            vertx,
            rpcClientFactory,
            configs.common,
            configs.ignoreTracesGeneratorErrors,
            configs.expectedTracesApiVersion,
            fallBackTracesCounters,
            log ?: LogManager.getLogger("clients.traces"),
          )
        TracesClients(tracesCountersClient = commonClient, tracesConflationClient = commonClient)
      }

      else -> {
        val countersClient =
          createTracesClient(
            vertx,
            rpcClientFactory,
            configs.counters!!,
            configs.ignoreTracesGeneratorErrors,
            configs.expectedTracesApiVersion,
            fallBackTracesCounters,
            log ?: LogManager.getLogger("clients.traces.counters"),
          )
        val conflationClient =
          createTracesClient(
            vertx,
            rpcClientFactory,
            configs.conflation!!,
            configs.ignoreTracesGeneratorErrors,
            configs.expectedTracesApiVersion,
            fallBackTracesCounters,
            log ?: LogManager.getLogger("clients.traces.conflation"),
          )
        TracesClients(tracesCountersClient = countersClient, tracesConflationClient = conflationClient)
      }
    }
  }
}

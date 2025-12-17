package net.consensys.linea.ethereum.gaspricing.staticcap

import com.github.michaelbull.result.Result
import com.github.michaelbull.result.onFailure
import com.github.michaelbull.result.onSuccess
import io.vertx.core.Future
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.ethereum.gaspricing.GasPriceUpdater
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.JsonRpcRequestFanOut
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URL
import java.util.concurrent.atomic.AtomicLong

class GenericGasPriceUpdater(
  httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  config: Config,
  private val setMinerGasPriceMethodName: String,
) : GasPriceUpdater {
  class Config(
    val endpoints: List<URL>,
    val retryConfig: RequestRetryConfig,
  ) {
    init {
      require(endpoints.isNotEmpty()) { "At least one endpoint is required" }
    }
  }

  private val log: Logger = LogManager.getLogger(this::class.java)
  private val id: AtomicLong = AtomicLong(0)

  private val setPriceRequestFanOut: JsonRpcRequestFanOut = createFanOutRpcClient(
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
    endpoints = config.endpoints,
    methodsToRetry = setOf(setMinerGasPriceMethodName),
    retryConfig = config.retryConfig,
    log = log,
  )

  override fun updateMinerGasPrice(gasPrice: ULong): SafeFuture<Unit> {
    return fireAndForget(gasPrice)
      .map {}
      .toSafeFuture()
  }

  private fun fireAndForget(gasPrice: ULong): Future<Unit> {
    val jsonRequest = JsonRpcRequestListParams(
      jsonrpc = "2.0",
      id = id.incrementAndGet(),
      method = setMinerGasPriceMethodName,
      params = listOf("0x${gasPrice.toString(16)}"),
    )

    return setPriceRequestFanOut.makeRequest(jsonRequest)
      .map(logResult(setMinerGasPriceMethodName))
      .onFailure { log.warn("Error from $setMinerGasPriceMethodName: json-rpc-error={}", it.message, it) }
  }

  private fun logResult(methodName: String): (Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>) -> Unit {
    return { result ->
      result.onSuccess {
        if (it.result != true) {
          log.warn("$methodName returned '{}'", it.result)
        }
      }.onFailure {
        log.warn("$methodName returned json-rpc-error={}", it.error)
      }
    }
  }

  companion object {
    private fun createFanOutRpcClient(
      httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
      endpoints: List<URL>,
      methodsToRetry: Set<String>,
      retryConfig: RequestRetryConfig,
      log: Logger,
    ): JsonRpcRequestFanOut {
      val rpcClients: List<JsonRpcClient> = endpoints.map { endpoint ->
        httpJsonRpcClientFactory.createWithRetries(
          endpoint = endpoint,
          retryConfig = retryConfig,
          methodsToRetry = methodsToRetry,
          log = log,
        )
      }
      return JsonRpcRequestFanOut(rpcClients)
    }
  }
}

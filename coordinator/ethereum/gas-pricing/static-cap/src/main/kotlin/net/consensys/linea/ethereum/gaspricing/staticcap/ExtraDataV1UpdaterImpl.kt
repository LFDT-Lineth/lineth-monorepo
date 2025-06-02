package net.consensys.linea.ethereum.gaspricing.staticcap

import com.github.michaelbull.result.onFailure
import com.github.michaelbull.result.onSuccess
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.ethereum.gaspricing.ExtraDataUpdater
import net.consensys.linea.ethereum.gaspricing.MinerExtraDataV1
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URL
import java.util.concurrent.atomic.AtomicLong

class ExtraDataV1UpdaterImpl(
  httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  config: Config,
) : ExtraDataUpdater {
  companion object {
    const val SET_MINER_EXTRA_DATA_METHOD_NAME = "miner_setExtraData"
  }

  data class Config(
    val sequencerEndpoint: URL,
    val retryConfig: RequestRetryConfig,
  )

  private val id: AtomicLong = AtomicLong(0)
  val log: Logger = LogManager.getLogger(this::class.java)

  private val setMinerExtraDataRpcClient: JsonRpcClient = httpJsonRpcClientFactory.createWithRetries(
    endpoint = config.sequencerEndpoint,
    methodsToRetry = setOf(SET_MINER_EXTRA_DATA_METHOD_NAME),
    retryConfig = config.retryConfig,
    log = log,
  )

  override fun updateMinerExtraData(extraData: MinerExtraDataV1): SafeFuture<Unit> {
    val jsonRequest = JsonRpcRequestListParams(
      jsonrpc = "2.0",
      id = id.incrementAndGet(),
      method = SET_MINER_EXTRA_DATA_METHOD_NAME,
      params = listOf(extraData.encode()),
    )
    return setMinerExtraDataRpcClient.makeRequest(jsonRequest)
      .toSafeFuture()
      .thenApply {
          result ->
        result.onSuccess {
          if (it.result != true) {
            log.warn("$SET_MINER_EXTRA_DATA_METHOD_NAME returned result='{}'", it.result)
          }
        }.onFailure {
          log.warn("$SET_MINER_EXTRA_DATA_METHOD_NAME returned json-rpc-error={}", it.error)
        }
      }
  }
}

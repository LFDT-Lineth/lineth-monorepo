package net.consensys.linea.ethereum.gaspricing.staticcap

import net.consensys.linea.ethereum.gaspricing.GasPriceUpdater
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URL

class GasPriceUpdaterImpl(
  private val httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  private val config: Config,
) : GasPriceUpdater {
  data class Config(
    val gethEndpoints: List<URL>,
    val besuEndPoints: List<URL>,
    val retryConfig: RequestRetryConfig,
  ) {
    init {
      require(gethEndpoints.isNotEmpty() || besuEndPoints.isNotEmpty()) {
        "Must have at least one geth or besu endpoint to update the gas price"
      }
    }
  }

  private val gethGasPriceUpdater: GenericGasPriceUpdater? = createPriceUpdater(
    endpoints = config.gethEndpoints,
    setMinerGasPriceMethodName = "miner_setGasPrice",
  )
  private val besuGasPriceUpdater: GenericGasPriceUpdater? = createPriceUpdater(
    endpoints = config.besuEndPoints,
    setMinerGasPriceMethodName = "miner_setMinGasPrice",
  )

  override fun updateMinerGasPrice(gasPrice: ULong): SafeFuture<Unit> {
    return SafeFuture.allOf(
      gethGasPriceUpdater?.updateMinerGasPrice(gasPrice) ?: SafeFuture.COMPLETE,
      besuGasPriceUpdater?.updateMinerGasPrice(gasPrice) ?: SafeFuture.COMPLETE,
    ).thenApply {}
  }

  private fun createPriceUpdater(
    endpoints: List<URL>,
    setMinerGasPriceMethodName: String,
  ): GenericGasPriceUpdater? {
    if (endpoints.isEmpty()) return null

    return GenericGasPriceUpdater(
      httpJsonRpcClientFactory = httpJsonRpcClientFactory,
      config = GenericGasPriceUpdater.Config(
        endpoints = endpoints,
        retryConfig = config.retryConfig,
      ),
      setMinerGasPriceMethodName = setMinerGasPriceMethodName,
    )
  }
}

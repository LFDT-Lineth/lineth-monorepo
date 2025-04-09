package net.consensys.zkevm.coordinator.app

import io.vertx.core.Vertx
import linea.web3j.Web3jBlobExtended
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.GasPriceUpdater
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1PricerService
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1UpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.GasPriceUpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.GasUsageRatioWeightedAverageFeesCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.MinMineableFeesPricerService
import net.consensys.linea.ethereum.gaspricing.staticcap.MinerExtraDataV1CalculatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.TransactionCostCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.VariableFeesCalculator
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.zkevm.LongRunningService
import org.apache.logging.log4j.LogManager
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import kotlin.time.Duration

class L2NetworkGasPricingService(
  vertx: Vertx,
  httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  l1Web3jClient: Web3j,
  l1Web3jService: Web3jBlobExtended,
  config: Config
) : LongRunningService {
  data class LegacyGasPricingCalculatorConfig(
    val transactionCostCalculatorConfig: TransactionCostCalculator.Config?,
    val naiveGasPricingCalculatorConfig: GasUsageRatioWeightedAverageFeesCalculator.Config?,
    val legacyGasPricingCalculatorBounds: BoundableFeeCalculator.Config
  )

  data class Config(
    val feeHistoryFetcherConfig: FeeHistoryFetcherImpl.Config,
    val legacy: LegacyGasPricingCalculatorConfig,
    val jsonRpcGasPriceUpdaterConfig: GasPriceUpdaterImpl.Config?,
    val jsonRpcPriceUpdateInterval: Duration,
    val extraDataPricingPropagationEnabled: Boolean,
    val extraDataUpdateInterval: Duration,
    val variableFeesCalculatorConfig: VariableFeesCalculator.Config,
    val variableFeesCalculatorBounds: BoundableFeeCalculator.Config,
    val extraDataCalculatorConfig: MinerExtraDataV1CalculatorImpl.Config,
    val extraDataUpdaterConfig: ExtraDataV1UpdaterImpl.Config
  )
  private val log = LogManager.getLogger(this::class.java)

  private val gasPricingFeesFetcher: FeesFetcher = FeeHistoryFetcherImpl(
    l1Web3jClient,
    l1Web3jService,
    config.feeHistoryFetcherConfig
  )

  private val boundedVariableCostCalculator = run {
    val variableCostCalculator = VariableFeesCalculator(
      config.variableFeesCalculatorConfig
    )
    BoundableFeeCalculator(
      config = config.variableFeesCalculatorBounds,
      feesCalculator = variableCostCalculator
    )
  }

  private val legacyGasPricingCalculator: FeesCalculator = run {
    val baseCalculator = if (config.legacy.transactionCostCalculatorConfig != null) {
      TransactionCostCalculator(boundedVariableCostCalculator, config.legacy.transactionCostCalculatorConfig)
    } else {
      GasUsageRatioWeightedAverageFeesCalculator(
        config.legacy.naiveGasPricingCalculatorConfig!!
      )
    }
    BoundableFeeCalculator(
      config.legacy.legacyGasPricingCalculatorBounds,
      baseCalculator
    )
  }

  private val minMineableFeesPricerService: MinMineableFeesPricerService? =
    if (config.jsonRpcGasPriceUpdaterConfig != null) {
      val l2SetGasPriceUpdater: GasPriceUpdater = GasPriceUpdaterImpl(
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
        config = config.jsonRpcGasPriceUpdaterConfig
      )

      MinMineableFeesPricerService(
        pollingInterval = config.jsonRpcPriceUpdateInterval,
        vertx = vertx,
        feesFetcher = gasPricingFeesFetcher,
        feesCalculator = legacyGasPricingCalculator,
        gasPriceUpdater = l2SetGasPriceUpdater
      )
    } else {
      null
    }

  private val extraDataPricerService: ExtraDataV1PricerService? = if (config.extraDataPricingPropagationEnabled) {
    ExtraDataV1PricerService(
      pollingInterval = config.extraDataUpdateInterval,
      vertx = vertx,
      feesFetcher = gasPricingFeesFetcher,
      minerExtraDataCalculatorImpl = MinerExtraDataV1CalculatorImpl(
        config = config.extraDataCalculatorConfig,
        variableFeesCalculator = boundedVariableCostCalculator,
        legacyFeesCalculator = legacyGasPricingCalculator
      ),
      extraDataUpdater = ExtraDataV1UpdaterImpl(
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
        config = config.extraDataUpdaterConfig
      )
    )
  } else {
    null
  }

  override fun start(): CompletableFuture<Unit> {
    return (minMineableFeesPricerService?.start() ?: SafeFuture.completedFuture(Unit))
      .thenCompose { extraDataPricerService?.start() ?: SafeFuture.completedFuture(Unit) }
      .thenPeek {
        log.info("GasPriceUpdater started")
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.allOf(
      minMineableFeesPricerService?.stop() ?: SafeFuture.completedFuture(Unit),
      extraDataPricerService?.stop() ?: SafeFuture.completedFuture(Unit)
    )
      .thenApply { }
      .thenPeek {
        log.info("GasPriceUpdater stopped")
      }
  }
}

package net.consensys.zkevm.coordinator.app

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.web3j.ExtendedWeb3J
import linea.web3j.Web3jBlobExtended
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.GasPriceUpdater
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1PricerService
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1UpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.GasPriceUpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.GasUsageRatioWeightedAverageFeesCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.HistoricVariableCostProviderImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.L2CalldataBasedVariableFeesCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.L2CalldataSizeAccumulatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.MinMineableFeesPricerService
import net.consensys.linea.ethereum.gaspricing.staticcap.MinerExtraDataV1CalculatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.TransactionCostCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.VariableFeesCalculator
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import kotlin.time.Duration

class L2NetworkGasPricingService(
  vertx: Vertx,
  metricsFacade: MetricsFacade,
  httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  l1Web3jClient: Web3j,
  l1Web3jService: Web3jBlobExtended,
  l2Web3jClient: ExtendedWeb3J,
  config: Config,
) : LongRunningService {
  data class LegacyGasPricingCalculatorConfig(
    val transactionCostCalculatorConfig: TransactionCostCalculator.Config?,
    val naiveGasPricingCalculatorConfig: GasUsageRatioWeightedAverageFeesCalculator.Config?,
    val legacyGasPricingCalculatorBounds: BoundableFeeCalculator.Config,
  )
  data class L2CalldataPricingConfig(
    val l2CalldataSizeAccumulatorConfig: L2CalldataSizeAccumulatorImpl.Config,
    val l2CalldataBasedVariableFeesCalculatorConfig: L2CalldataBasedVariableFeesCalculator.Config,
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
    val extraDataUpdaterConfig: ExtraDataV1UpdaterImpl.Config,
    val l2CalldataPricingCalculatorConfig: L2CalldataPricingConfig?,
  )
  private val log = LogManager.getLogger(this::class.java)

  private val gasPricingFeesFetcher: FeesFetcher = FeeHistoryFetcherImpl(
    l1Web3jClient,
    l1Web3jService,
    config.feeHistoryFetcherConfig,
  )

  private fun isL2CalldataBasedVariableFeesEnabled(config: Config): Boolean {
    return config.l2CalldataPricingCalculatorConfig != null
  }

  private val variableCostCalculator = run {
    val boundedVariableCostCalculator = run {
      val variableCostCalculator = VariableFeesCalculator(
        config.variableFeesCalculatorConfig,
      )
      BoundableFeeCalculator(
        config = config.variableFeesCalculatorBounds,
        feesCalculator = variableCostCalculator,
      )
    }

    if (isL2CalldataBasedVariableFeesEnabled(config)) {
      val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
        web3jClient = l2Web3jClient,
        config = config.l2CalldataPricingCalculatorConfig!!.l2CalldataSizeAccumulatorConfig,
      )
      val historicVariableCostProvider = HistoricVariableCostProviderImpl(
        web3jClient = l2Web3jClient,
      )
      L2CalldataBasedVariableFeesCalculator(
        web3jClient = l2Web3jClient,
        variableFeesCalculator = boundedVariableCostCalculator,
        l2CalldataSizeAccumulator = l2CalldataSizeAccumulator,
        historicVariableCostProvider = historicVariableCostProvider,
        config = config.l2CalldataPricingCalculatorConfig.l2CalldataBasedVariableFeesCalculatorConfig,
      )
    } else {
      boundedVariableCostCalculator
    }
  }

  private val legacyGasPricingCalculator = run {
    val baseCalculator = if (config.legacy.transactionCostCalculatorConfig != null) {
      TransactionCostCalculator(variableCostCalculator, config.legacy.transactionCostCalculatorConfig)
    } else {
      GasUsageRatioWeightedAverageFeesCalculator(
        config.legacy.naiveGasPricingCalculatorConfig!!,
      )
    }
    if (isL2CalldataBasedVariableFeesEnabled(config)) {
      baseCalculator
    } else {
      BoundableFeeCalculator(
        config.legacy.legacyGasPricingCalculatorBounds,
        baseCalculator,
      )
    }
  }

  private val minMineableFeesPricerService: MinMineableFeesPricerService? =
    if (config.jsonRpcGasPriceUpdaterConfig != null) {
      val l2SetGasPriceUpdater: GasPriceUpdater = GasPriceUpdaterImpl(
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
        config = config.jsonRpcGasPriceUpdaterConfig,
      )

      MinMineableFeesPricerService(
        pollingInterval = config.jsonRpcPriceUpdateInterval,
        vertx = vertx,
        feesFetcher = gasPricingFeesFetcher,
        feesCalculator = legacyGasPricingCalculator,
        gasPriceUpdater = l2SetGasPriceUpdater,
      )
    } else {
      null
    }

  private val extraDataPricerService: ExtraDataV1PricerService? = if (config.extraDataPricingPropagationEnabled) {
    ExtraDataV1PricerService(
      pollingInterval = config.extraDataUpdateInterval,
      vertx = vertx,
      feesFetcher = gasPricingFeesFetcher,
      minerExtraDataCalculator = MinerExtraDataV1CalculatorImpl(
        config = config.extraDataCalculatorConfig,
        variableFeesCalculator = variableCostCalculator,
        legacyFeesCalculator = legacyGasPricingCalculator,
      ),
      extraDataUpdater = ExtraDataV1UpdaterImpl(
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
        config = config.extraDataUpdaterConfig,
      ),
      metricsFacade = metricsFacade,
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
      extraDataPricerService?.stop() ?: SafeFuture.completedFuture(Unit),
    )
      .thenApply { }
      .thenPeek {
        log.info("GasPriceUpdater stopped")
      }
  }
}

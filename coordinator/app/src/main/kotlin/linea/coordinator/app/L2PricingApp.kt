package linea.coordinator.app

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.coordinator.config.toJsonRpcRetry
import linea.coordinator.config.v2.L2NetworkGasPricingConfig
import linea.ethapi.EthApiClient
import linea.kotlin.toKWeiUInt
import linea.web3j.ethapi.createEthApiClient
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1UpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.L2CalldataBasedVariableFeesCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.L2CalldataSizeAccumulatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.MinerExtraDataV1CalculatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.TransactionCostCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.VariableFeesCalculator
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import java.util.concurrent.CompletableFuture
import kotlin.time.Duration

/**
 * Responsibilities:
 *  - compute and propagate L2 network gas pricing (legacy + extra-data based)
 */
class L2PricingApp(
  private val l2NetworkGasPricingConfig: L2NetworkGasPricingConfig,
  private val vertx: Vertx,
  private val metricsFacade: MetricsFacade,
  private val httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  private val l2EthApiClient: EthApiClient = createEthApiClient(
    rpcUrl = l2NetworkGasPricingConfig.l2Endpoint.toString(),
    log = LogManager.getLogger("clients.l2.eth.l2pricing"),
    vertx = vertx,
    requestRetryConfig = l2NetworkGasPricingConfig.l2RequestRetries,
  ),
  private val l1EthApiClient: EthApiClient = createEthApiClient(
    rpcUrl = l2NetworkGasPricingConfig.l1Endpoint.toString(),
    log = LogManager.getLogger("clients.l1.eth.l1pricing"),
    vertx = vertx,
    requestRetryConfig = l2NetworkGasPricingConfig.l1RequestRetries,
  ),
) : LongRunningService {
  private val log = LogManager.getLogger(this::class.java)

  private val l2NetworkGasPricingService: L2NetworkGasPricingService = run {
    val legacyConfig = L2NetworkGasPricingService.LegacyGasPricingCalculatorConfig(
      transactionCostCalculatorConfig = TransactionCostCalculator.Config(
        sampleTransactionCostMultiplier = l2NetworkGasPricingConfig.flatRateGasPricing.plainTransferCostMultiplier,
        fixedCostWei = l2NetworkGasPricingConfig.gasPriceFixedCost,
        compressedTxSize = l2NetworkGasPricingConfig.flatRateGasPricing.compressedTxSize.toInt(),
        expectedGas = l2NetworkGasPricingConfig.flatRateGasPricing.expectedGas.toInt(),
      ),
      naiveGasPricingCalculatorConfig = null,
      legacyGasPricingCalculatorBounds = BoundableFeeCalculator.Config(
        feeUpperBound = l2NetworkGasPricingConfig.flatRateGasPricing.gasPriceUpperBound.toDouble(),
        feeLowerBound = l2NetworkGasPricingConfig.flatRateGasPricing.gasPriceLowerBound.toDouble(),
        feeMargin = 0.0,
      ),
    )

    val config = L2NetworkGasPricingService.Config(
      feeHistoryFetcherConfig = FeeHistoryFetcherImpl.Config(
        feeHistoryBlockCount = l2NetworkGasPricingConfig.feeHistoryBlockCount,
        feeHistoryRewardPercentile = l2NetworkGasPricingConfig.feeHistoryRewardPercentile.toDouble(),
      ),
      legacy = legacyConfig,
      jsonRpcGasPriceUpdaterConfig = null,
      // we do not use miner_setGasPrice RPC method, so we set it to infinite
      jsonRpcPriceUpdateInterval = Duration.INFINITE,
      // there no other way to work now without setting extra data into sequencer node
      extraDataPricingPropagationEnabled = true,
      extraDataUpdateInterval = l2NetworkGasPricingConfig.priceUpdateInterval,
      variableFeesCalculatorConfig = VariableFeesCalculator.Config(
        blobSubmissionExpectedExecutionGas = l2NetworkGasPricingConfig.dynamicGasPricing
          .blobSubmissionExpectedExecutionGas.toUInt(),
        bytesPerDataSubmission = l2NetworkGasPricingConfig.dynamicGasPricing.l1BlobGas.toUInt(),
        expectedBlobGas = l2NetworkGasPricingConfig.dynamicGasPricing.l1BlobGas.toUInt(),
        margin = l2NetworkGasPricingConfig.dynamicGasPricing.margin,
      ),
      variableFeesCalculatorBounds = BoundableFeeCalculator.Config(
        feeUpperBound = l2NetworkGasPricingConfig.dynamicGasPricing.variableCostUpperBound.toDouble(),
        feeLowerBound = l2NetworkGasPricingConfig.dynamicGasPricing.variableCostLowerBound.toDouble(),
        feeMargin = 0.0,
      ),
      extraDataCalculatorConfig = MinerExtraDataV1CalculatorImpl.Config(
        fixedCostInKWei = l2NetworkGasPricingConfig.gasPriceFixedCost.toKWeiUInt(),
        ethGasPriceMultiplier = 1.0,
      ),
      extraDataUpdaterConfig = ExtraDataV1UpdaterImpl.Config(
        sequencerEndpoint = l2NetworkGasPricingConfig.extraDataUpdateEndpoint,
        retryConfig = l2NetworkGasPricingConfig.extraDataUpdateRequestRetries.toJsonRpcRetry(),
      ),
      l2CalldataPricingCalculatorConfig = l2NetworkGasPricingConfig.dynamicGasPricing.calldataBasedPricing?.let {
        if (l2NetworkGasPricingConfig.dynamicGasPricing.calldataBasedPricing.calldataSumSizeBlockCount > 0U) {
          L2NetworkGasPricingService.L2CalldataPricingConfig(
            l2CalldataSizeAccumulatorConfig = L2CalldataSizeAccumulatorImpl.Config(
              blockSizeNonCalldataOverhead = it.blockSizeNonCalldataOverhead,
              calldataSizeBlockCount = it.calldataSumSizeBlockCount,
            ),
            l2CalldataBasedVariableFeesCalculatorConfig = L2CalldataBasedVariableFeesCalculator.Config(
              feeChangeDenominator = it.feeChangeDenominator,
              calldataSizeBlockCount = it.calldataSumSizeBlockCount,
              maxBlockCalldataSize = it.calldataSumSizeTarget.toUInt(),
            ),
          )
        } else {
          log.debug("Calldata-based variable fee is disabled as calldataSizeBlockCount is set as 0")
          null
        }
      },
    )
    L2NetworkGasPricingService(
      vertx = vertx,
      metricsFacade = metricsFacade,
      httpJsonRpcClientFactory = httpJsonRpcClientFactory,
      l1EthApiClient = l1EthApiClient,
      l2EthApiBlockClient = l2EthApiClient,
      config = config,
    )
  }

  override fun start(): CompletableFuture<Unit> {
    return l2NetworkGasPricingService.start()
  }

  override fun stop(): CompletableFuture<Unit> {
    return l2NetworkGasPricingService.stop()
  }
}

package net.consensys.zkevm.coordinator.app.config

import com.sksamuel.hoplite.ConfigAlias
import linea.kotlin.toKWeiUInt
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1UpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.GasPriceUpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.GasUsageRatioWeightedAverageFeesCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.MinerExtraDataV1CalculatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.TransactionCostCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.VariableFeesCalculator
import net.consensys.zkevm.coordinator.app.L2NetworkGasPricingService
import java.net.URL
import java.time.Duration
import kotlin.time.toKotlinDuration

// Defaults to a compressed plain transfer transaction
data class SampleTransactionGasPricingTomlDto(
  val plainTransferCostMultiplier: Double = 1.0,
  val compressedTxSize: Int = 125,
  val expectedGas: Int = 21000
)

data class LegacyGasPricingTomlDto(
  val type: Type,

  val naiveGasPricing: NaiveGasPricingTomlDto?,
  val sampleTransactionGasPricing: SampleTransactionGasPricingTomlDto = SampleTransactionGasPricingTomlDto(),
  val gasPriceUpperBound: ULong,
  val gasPriceLowerBound: ULong
) {
  enum class Type {
    Naive,
    SampleTransaction
  }

  init {
    if (type == Type.Naive && naiveGasPricing == null) {
      throw IllegalStateException("LegacyGasPricing $type configuration is null.")
    }

    require(gasPriceUpperBound >= gasPriceLowerBound) {
      "gasPriceUpperBound must be greater than or equal to gasPriceLowerBound"
    }
  }
}

data class NaiveGasPricingTomlDto(
  val baseFeeCoefficient: Double,
  val priorityFeeCoefficient: Double,
  val baseFeeBlobCoefficient: Double
)

data class VariableCostPricingTomlDto(
  val gasPriceFixedCost: ULong,
  val legacyFeesMultiplier: Double,
  val margin: Double,
  val variableCostUpperBound: ULong,
  val variableCostLowerBound: ULong
) {
  init {
    require(variableCostUpperBound >= variableCostLowerBound) {
      "variableCostUpperBound must be greater than or equal to variableCostLowerBound"
    }
  }
}

data class JsonRpcPricingPropagationTomlDto(
  override var disabled: Boolean = false,
  val gethGasPriceUpdateRecipients: List<URL>,
  val besuGasPriceUpdateRecipients: List<URL>
) : FeatureToggleable {
  init {
    require(disabled || (gethGasPriceUpdateRecipients.isNotEmpty() || besuGasPriceUpdateRecipients.isNotEmpty())) {
      "There is no point of enabling JSON RPC pricing propagation if there are no " +
        "gethGasPriceUpdateRecipients or besuGasPriceUpdateRecipients defined"
    }
  }
}

data class ExtraDataPricingPropagationTomlDto(
  override var disabled: Boolean = false,
  val extraDataUpdateRecipient: URL
) : FeatureToggleable

data class L2NetworkGasPricingTomlDto(
  override var disabled: Boolean = false,
  override val requestRetry: RequestRetryConfigTomlFriendly,

  val priceUpdateInterval: Duration,
  val feeHistoryBlockCount: Int,
  val feeHistoryRewardPercentile: Double,

  val blobSubmissionExpectedExecutionGas: Int,
  @ConfigAlias("bytesPerDataSubmission") val _bytesPerDataSubmission: Int?,
  val l1BlobGas: Int,

  val legacy: LegacyGasPricingTomlDto,
  val variableCostPricing: VariableCostPricingTomlDto,
  val jsonRpcPricingPropagation: JsonRpcPricingPropagationTomlDto?,
  val extraDataPricingPropagation: ExtraDataPricingPropagationTomlDto
) : FeatureToggleable, RequestRetryConfigurable {
  init {
    require(feeHistoryBlockCount > 0) { "feeHistoryBlockCount must be greater than 0" }
    require(blobSubmissionExpectedExecutionGas > 0) { "blobSubmissionExpectedExecutionGas must be greater than 0" }
    require(l1BlobGas > 0) { "l1BlobGas must be greater than 0" }

    require(disabled || (jsonRpcPricingPropagation?.enabled == true || extraDataPricingPropagation.enabled)) {
      "There is no point of enabling L2 network gas pricing if " +
        "both jsonRpcPricingPropagation and extraDataPricingPropagation are disabled"
    }
  }

  private val bytesPerDataSubmission = _bytesPerDataSubmission ?: l1BlobGas

  fun reified(): L2NetworkGasPricingService.Config {
    val legacyGasPricingConfig = when (legacy.type) {
      LegacyGasPricingTomlDto.Type.Naive -> {
        L2NetworkGasPricingService.LegacyGasPricingCalculatorConfig(
          transactionCostCalculatorConfig = null,
          naiveGasPricingCalculatorConfig = GasUsageRatioWeightedAverageFeesCalculator.Config(
            baseFeeCoefficient = legacy.naiveGasPricing!!.baseFeeCoefficient,
            priorityFeeCoefficient = legacy.naiveGasPricing.priorityFeeCoefficient,
            baseFeeBlobCoefficient = legacy.naiveGasPricing.baseFeeBlobCoefficient,
            blobSubmissionExpectedExecutionGas = blobSubmissionExpectedExecutionGas,
            expectedBlobGas = l1BlobGas
          ),
          legacyGasPricingCalculatorBounds = BoundableFeeCalculator.Config(
            legacy.gasPriceUpperBound.toDouble(),
            legacy.gasPriceLowerBound.toDouble(),
            0.0
          )
        )
      }

      LegacyGasPricingTomlDto.Type.SampleTransaction -> {
        L2NetworkGasPricingService.LegacyGasPricingCalculatorConfig(
          transactionCostCalculatorConfig = TransactionCostCalculator.Config(
            sampleTransactionCostMultiplier = legacy.sampleTransactionGasPricing.plainTransferCostMultiplier,
            fixedCostWei = variableCostPricing.gasPriceFixedCost,
            compressedTxSize = legacy.sampleTransactionGasPricing.compressedTxSize,
            expectedGas = legacy.sampleTransactionGasPricing.expectedGas
          ),
          naiveGasPricingCalculatorConfig = null,
          legacyGasPricingCalculatorBounds = BoundableFeeCalculator.Config(
            legacy.gasPriceUpperBound.toDouble(),
            legacy.gasPriceLowerBound.toDouble(),
            0.0
          )
        )
      }
    }
    val gasPriceUpdaterConfig = if (jsonRpcPricingPropagation?.enabled == true) {
      GasPriceUpdaterImpl.Config(
        gethEndpoints = jsonRpcPricingPropagation.gethGasPriceUpdateRecipients,
        besuEndPoints = jsonRpcPricingPropagation.besuGasPriceUpdateRecipients,
        retryConfig = requestRetryConfig
      )
    } else {
      null
    }
    return L2NetworkGasPricingService.Config(
      feeHistoryFetcherConfig = FeeHistoryFetcherImpl.Config(
        feeHistoryBlockCount = feeHistoryBlockCount.toUInt(),
        feeHistoryRewardPercentile = feeHistoryRewardPercentile
      ),
      legacy = legacyGasPricingConfig,
      jsonRpcGasPriceUpdaterConfig = gasPriceUpdaterConfig,
      jsonRpcPriceUpdateInterval = priceUpdateInterval.toKotlinDuration(),
      extraDataPricingPropagationEnabled = extraDataPricingPropagation.enabled,
      extraDataUpdateInterval = priceUpdateInterval.toKotlinDuration(),
      variableFeesCalculatorConfig = VariableFeesCalculator.Config(
        blobSubmissionExpectedExecutionGas = blobSubmissionExpectedExecutionGas.toUInt(),
        bytesPerDataSubmission = l1BlobGas.toUInt(),
        expectedBlobGas = bytesPerDataSubmission.toUInt(),
        margin = variableCostPricing.margin
      ),
      variableFeesCalculatorBounds = BoundableFeeCalculator.Config(
        feeUpperBound = variableCostPricing.variableCostUpperBound.toDouble(),
        feeLowerBound = variableCostPricing.variableCostLowerBound.toDouble(),
        feeMargin = 0.0
      ),
      extraDataCalculatorConfig = MinerExtraDataV1CalculatorImpl.Config(
        fixedCostInKWei = variableCostPricing.gasPriceFixedCost.toKWeiUInt(),
        ethGasPriceMultiplier = variableCostPricing.legacyFeesMultiplier
      ),
      extraDataUpdaterConfig = ExtraDataV1UpdaterImpl.Config(
        extraDataPricingPropagation.extraDataUpdateRecipient,
        requestRetryConfig
      )
    )
  }
}

package linea.web3j.gas

import linea.kotlin.toBigInteger
import linea.kotlin.toIntervalString
import linea.web3j.domain.blocksRange
import linea.web3j.domain.toLineaDomain
import linea.web3j.requestAsync
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameterName
import java.math.BigInteger
import kotlin.math.min

class EIP1559GasProvider(private val web3jClient: Web3j, private val config: Config) :
  AtomicContractEIP1559GasProvider {
  private val log: Logger = LogManager.getLogger(this::class.java)

  data class Config(
    val gasLimit: ULong,
    val maxFeePerGasCap: ULong,
    val feeHistoryBlockCount: UInt,
    val feeHistoryRewardPercentile: Double,
  ) {
    init {
      require(feeHistoryBlockCount > 0u) {
        "feeHistoryBlockCount=$feeHistoryBlockCount must be greater than 0."
      }
    }
  }

  private val chainId: Long = web3jClient.ethChainId().send().chainId.toLong()
  private var cacheIsValidForBlockNumber: BigInteger = BigInteger.ZERO
  private var feesCache: EIP1559GasFees = getRecentFees()
  private var gasLimitOverride = BigInteger.valueOf(Long.MAX_VALUE)

  private fun getRecentFees(): EIP1559GasFees {
    val currentBlockNumber = web3jClient.ethBlockNumber().send().blockNumber
    if (currentBlockNumber > cacheIsValidForBlockNumber) {
      web3jClient
        .ethFeeHistory(
          config.feeHistoryBlockCount.toInt(),
          DefaultBlockParameterName.LATEST,
          listOf(config.feeHistoryRewardPercentile),
        )
        .requestAsync { feeHistoryResponse ->
          val feeHistory = feeHistoryResponse.feeHistory.toLineaDomain()
          var maxPriorityFeePerGas = feeHistory.reward.sumOf { it[0] } / feeHistory.reward.size.toUInt()

          if (maxPriorityFeePerGas > config.maxFeePerGasCap) {
            maxPriorityFeePerGas = config.maxFeePerGasCap
            log.warn(
              "Estimated miner tip of $maxPriorityFeePerGas exceeds configured max " +
                "fee per gas of ${config.maxFeePerGasCap} returning cap instead!",
            )
          }

          cacheIsValidForBlockNumber = currentBlockNumber

          val maxFeePerGas = (feeHistory.baseFeePerGas.last() * 2uL) + maxPriorityFeePerGas

          if (maxFeePerGas > 0uL && maxPriorityFeePerGas > 0uL) {
            feesCache = EIP1559GasFees(
              maxPriorityFeePerGas = maxPriorityFeePerGas,
              maxFeePerGas = min(maxFeePerGas, config.maxFeePerGasCap),
            )
            log.debug(
              "New fees estimation: fees={} l2Blocks={}",
              feeHistoryResponse.feeHistory.blocksRange().toIntervalString(),
              feesCache,
            )
          } else {
            feesCache = EIP1559GasFees(
              maxPriorityFeePerGas = 0uL,
              maxFeePerGas = config.maxFeePerGasCap,
            )
          }
        }
        .get()
    }
    return feesCache
  }

  override fun getGasPrice(contractFunc: String?): BigInteger {
    throw NotImplementedError("EIP1559GasProvider only implements EIP1559 specific methods")
  }

  @Deprecated("Deprecated in Java")
  override fun getGasPrice(): BigInteger {
    throw NotImplementedError("EIP1559GasProvider only implements EIP1559 specific methods")
  }

  fun overrideNextGasLimit(gasLimit: BigInteger) {
    gasLimitOverride = gasLimit
  }

  private fun resetGasLimitOverride() {
    gasLimitOverride = BigInteger.valueOf(Long.MAX_VALUE)
  }

  override fun getGasLimit(contractFunc: String?): BigInteger {
    val gasLimit = gasLimitOverride.min(config.gasLimit.toBigInteger())
    resetGasLimitOverride()

    return gasLimit
  }

  @Deprecated("Deprecated in Java")
  override fun getGasLimit(): BigInteger {
    return config.gasLimit.toBigInteger()
  }

  override fun isEIP1559Enabled(): Boolean {
    return true
  }

  override fun getChainId(): Long {
    return chainId
  }

  override fun getMaxFeePerGas(contractFunc: String?): BigInteger {
    return getRecentFees().maxFeePerGas.toBigInteger()
  }

  override fun getMaxPriorityFeePerGas(contractFunc: String?): BigInteger {
    return getRecentFees().maxPriorityFeePerGas.toBigInteger()
  }

  override fun getEIP1559GasFees(): EIP1559GasFees {
    return getRecentFees()
  }
}

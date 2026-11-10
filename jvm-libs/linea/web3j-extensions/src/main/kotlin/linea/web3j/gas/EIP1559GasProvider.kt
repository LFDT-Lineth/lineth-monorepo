package linea.web3j.gas

import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.kotlin.toBigInteger
import linea.kotlin.toIntervalString
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.core.methods.request.Transaction
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import kotlin.math.min

class EIP1559GasProvider(private val ethApiClient: EthApiClient, private val config: Config) :
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

  private val chainId: Long = ethApiClient.ethChainId().get().toLong()
  private var cacheIsValidForBlockNumber: BigInteger = BigInteger.ZERO
  private var feesCache: EIP1559GasFees = getRecentFees()

  private fun getRecentFees(): EIP1559GasFees {
    // the future yields the uninitialized cache when called from the initializer at block 0
    getRecentFeesAsync().get()
    return feesCache
  }

  private fun getRecentFeesAsync(): SafeFuture<EIP1559GasFees> {
    return ethApiClient.ethBlockNumber().thenCompose { blockNumber ->
      val currentBlockNumber = blockNumber.toBigInteger()
      if (currentBlockNumber > cacheIsValidForBlockNumber) {
        fetchFees(currentBlockNumber)
      } else {
        SafeFuture.completedFuture(feesCache)
      }
    }
  }

  private fun fetchFees(currentBlockNumber: BigInteger): SafeFuture<EIP1559GasFees> {
    return ethApiClient
      .ethFeeHistory(
        config.feeHistoryBlockCount.toInt(),
        BlockParameter.Tag.LATEST,
        listOf(config.feeHistoryRewardPercentile),
      )
      .thenApply { feeHistory ->
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
            feeHistory.blocksRange().toIntervalString(),
            feesCache,
          )
        } else {
          feesCache = EIP1559GasFees(
            maxPriorityFeePerGas = 0uL,
            maxFeePerGas = config.maxFeePerGasCap,
          )
        }
        feesCache
      }
  }

  @Deprecated("Deprecated in Java")
  override fun getGasPrice(): BigInteger {
    throw NotImplementedError("EIP1559GasProvider only implements EIP1559 specific methods")
  }

  override fun getGasLimit(transaction: Transaction): BigInteger {
    return config.gasLimit.toBigInteger()
  }

  @Deprecated("Deprecated in Java")
  override fun getGasLimit(): BigInteger {
    return config.gasLimit.toBigInteger()
  }

  override fun getChainId(): Long {
    return chainId
  }

  override fun getMaxFeePerGas(): BigInteger {
    return getRecentFees().maxFeePerGas.toBigInteger()
  }

  override fun getMaxPriorityFeePerGas(): BigInteger {
    return getRecentFees().maxPriorityFeePerGas.toBigInteger()
  }

  override fun getEIP1559GasFees(): EIP1559GasFees {
    return getRecentFees()
  }

  override fun getEIP1559GasFeesAsync(): SafeFuture<EIP1559GasFees> {
    return getRecentFeesAsync()
  }
}

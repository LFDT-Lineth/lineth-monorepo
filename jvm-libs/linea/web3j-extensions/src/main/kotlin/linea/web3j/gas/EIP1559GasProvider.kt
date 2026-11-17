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
import java.util.concurrent.atomic.AtomicReference
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

  private data class CachedFees(val blockNumber: ULong, val fees: EIP1559GasFees)

  private val chainId: Long = ethApiClient.ethChainId().get().toLong()

  // Block number and fees live in one immutable value, so they are always read and replaced together.
  private val cache = AtomicReference<CachedFees?>(null)

  private fun getRecentFees(): SafeFuture<EIP1559GasFees> {
    return ethApiClient.ethBlockNumber().thenCompose { blockNumber ->
      val cached = cache.get()
      if (cached != null && cached.blockNumber >= blockNumber) {
        SafeFuture.completedFuture(cached.fees)
      } else {
        fetchFees().thenPeek { fees ->
          // concurrent fetches can complete out of order: never replace newer fees with older ones
          val fetched = CachedFees(blockNumber, fees)
          cache.updateAndGet { current ->
            if (current == null || fetched.blockNumber > current.blockNumber) fetched else current
          }
        }
      }
    }
  }

  private fun fetchFees(): SafeFuture<EIP1559GasFees> {
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

        val maxFeePerGas = (feeHistory.baseFeePerGas.last() * 2uL) + maxPriorityFeePerGas

        if (maxFeePerGas > 0uL && maxPriorityFeePerGas > 0uL) {
          EIP1559GasFees(
            maxPriorityFeePerGas = maxPriorityFeePerGas,
            maxFeePerGas = min(maxFeePerGas, config.maxFeePerGasCap),
          ).also {
            log.debug(
              "New fees estimation: fees={} l2Blocks={}",
              feeHistory.blocksRange().toIntervalString(),
              it,
            )
          }
        } else {
          EIP1559GasFees(
            maxPriorityFeePerGas = 0uL,
            maxFeePerGas = config.maxFeePerGasCap,
          )
        }
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
    return getRecentFees().get().maxFeePerGas.toBigInteger()
  }

  override fun getMaxPriorityFeePerGas(): BigInteger {
    return getRecentFees().get().maxPriorityFeePerGas.toBigInteger()
  }

  override fun getEIP1559GasFees(): SafeFuture<EIP1559GasFees> {
    return getRecentFees()
  }
}

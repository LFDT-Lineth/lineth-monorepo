package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.BlockInterval
import linea.kotlin.toBigInteger
import linea.web3j.ExtendedWeb3J
import net.consensys.linea.ethereum.gaspricing.L2CalldataSizeAccumulator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.atomic.AtomicReference

class L2CalldataSizeAccumulatorImpl(
  private val web3jClient: ExtendedWeb3J,
  private val config: Config,
) : L2CalldataSizeAccumulator {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private var lastCalldataSizeSum: AtomicReference<Pair<ULong, BigInteger>> =
    AtomicReference(0UL to BigInteger.ZERO)

  data class Config(
    val blockSizeNonCalldataOverhead: UInt,
    val calldataSizeBlockCount: UInt,
  ) {
    init {
      require(calldataSizeBlockCount <= 60u) {
        "calldataSizeBlockCount must be less than 60 to avoid excessive " +
          "eth_getBlockByNumber calls to the web3j client. Value=$calldataSizeBlockCount"
      }
    }
  }

  override fun getSumOfL2CalldataSize(blockNumber: ULong): SafeFuture<BigInteger> {
    val (cachedBlockNumber, cachedCalldataSizeSum) = lastCalldataSizeSum.get()
    return if (cachedBlockNumber == blockNumber) {
      log.debug(
        "Use cached lastCalldataSizeSum={} latestBlockNumber={}",
        cachedCalldataSizeSum,
        blockNumber,
      )
      SafeFuture.completedFuture(cachedCalldataSizeSum)
    } else if (config.calldataSizeBlockCount > 0u && blockNumber >= config.calldataSizeBlockCount.toULong()) {
      val blockRange = (blockNumber - config.calldataSizeBlockCount + 1U)..blockNumber
      val futures = blockRange.map { blockNumber ->
        web3jClient.ethGetBlockSizeByNumber(blockNumber.toLong())
      }
      SafeFuture.collectAll(futures.stream())
        .thenApply { blockSizes ->
          blockSizes.sumOf {
            it.minus(config.blockSizeNonCalldataOverhead.toULong().toBigInteger())
              .coerceAtLeast(BigInteger.ZERO)
          }.also { calldataSizeSum ->
            log.debug(
              "sumOfBlockSizes={} blockSizes={} blockRange={} blockSizeNonCalldataOverhead={}",
              calldataSizeSum,
              blockSizes,
              BlockInterval.between(blockRange.start, blockRange.last).intervalString(),
              config.blockSizeNonCalldataOverhead,

            )
            lastCalldataSizeSum.set(blockNumber to calldataSizeSum)
          }
        }
    } else {
      SafeFuture.completedFuture(BigInteger.ZERO)
    }
      .whenException { th ->
        log.error(
          "Get the sum of L2 calldata size from the last {} blocks failure: {}",
          config.calldataSizeBlockCount,
          th.message,
          th,
        )
      }
  }
}

package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.BlockInterval
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.ethapi.EthApiBlockClient
import net.consensys.linea.ethereum.gaspricing.L2CalldataSizeAccumulator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

class L2CalldataSizeAccumulatorImpl(
  private val ethApiBlockClient: EthApiBlockClient,
  private val config: Config,
) : L2CalldataSizeAccumulator {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private var lastCalldataSizeSum: AtomicReference<Pair<ULong, ULong>> =
    AtomicReference(0UL to 0uL)

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

  override fun getSumOfL2CalldataSize(blockNumber: ULong): SafeFuture<ULong> {
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
        ethApiBlockClient.ethGetBlockByNumberTxHashes(blockNumber.toBlockParameter())
          .thenApply {
            it.size
          }
      }
      SafeFuture.collectAll(futures.stream())
        .thenApply { blockSizes ->
          blockSizes.sumOf {
            if (it >= config.blockSizeNonCalldataOverhead.toULong()) {
              it - config.blockSizeNonCalldataOverhead.toULong()
            } else {
              0uL
            }
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
      SafeFuture.completedFuture(0uL)
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

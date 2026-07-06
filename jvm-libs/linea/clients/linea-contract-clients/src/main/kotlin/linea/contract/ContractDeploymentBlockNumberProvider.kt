package linea.contract

import linea.contract.events.Upgraded
import linea.domain.BlockParameter
import linea.domain.toBlockParameter
import linea.ethapi.EthApiClient
import linea.ethapi.extensions.getAbsoluteBlockNumbers
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

typealias ContractDeploymentBlockNumberProvider = () -> SafeFuture<ULong>

class StaticContractDeploymentBlockNumberProvider(
  private val deploymentBlockNumber: ULong,
) : ContractDeploymentBlockNumberProvider {
  override fun invoke(): SafeFuture<ULong> {
    return SafeFuture.completedFuture(deploymentBlockNumber)
  }
}

class EventBasedContractDeploymentBlockNumberProvider(
  private val ethApiClient: EthApiClient,
  private val contractAddress: String,
  private val ethLogsSearchMaxBlockRange: UInt = 10_000u,
  private val log: Logger = LogManager.getLogger(EventBasedContractDeploymentBlockNumberProvider::class.java),
) : ContractDeploymentBlockNumberProvider {
  private val deploymentBlockNumberCache = AtomicReference<ULong>(0UL)

  fun getDeploymentBlock(): SafeFuture<ULong> {
    if (deploymentBlockNumberCache.get() != 0UL) {
      return SafeFuture.completedFuture(deploymentBlockNumberCache.get())
    }
    // The deployment block is the first block that emitted an Upgraded event. Scan forward in bounded
    // chunks instead of a single getLogs(EARLIEST..LATEST), which rate-limited providers (e.g. Infura)
    // reject for spans > 10_000 blocks. The result is cached, so this runs at most once.
    return ethApiClient
      .getAbsoluteBlockNumbers(BlockParameter.Tag.EARLIEST, BlockParameter.Tag.LATEST)
      .thenCompose { (start, end) -> findFirstUpgradedBlock(fromBlock = start, toBlock = end) }
      .thenApply { blockNumber ->
        blockNumber ?: throw IllegalStateException("Upgraded event not found: contractAddress=$contractAddress")
      }
      .thenPeek { deploymentBlockNumberCache.set(it) }
      .whenException {
        log.error(
          "Failed to get deployment block number for contract={} errorMessage={}",
          contractAddress,
          it.message,
        )
      }
  }

  private fun findFirstUpgradedBlock(fromBlock: ULong, toBlock: ULong): SafeFuture<ULong?> {
    if (fromBlock > toBlock) {
      return SafeFuture.completedFuture(null)
    }
    val chunkEnd = minOf(fromBlock + ethLogsSearchMaxBlockRange.toULong() - 1UL, toBlock)
    return ethApiClient
      .getLogs(
        fromBlock = fromBlock.toBlockParameter(),
        toBlock = chunkEnd.toBlockParameter(),
        address = contractAddress,
        topics = listOf(Upgraded.topic),
      ).thenCompose { logs ->
        // Scanning forward, so the first chunk with any Upgraded event holds the earliest one.
        val earliestInChunk = logs.minByOrNull { it.blockNumber }
        if (earliestInChunk != null) {
          SafeFuture.completedFuture(earliestInChunk.blockNumber)
        } else {
          findFirstUpgradedBlock(fromBlock = chunkEnd + 1UL, toBlock = toBlock)
        }
      }
  }

  override fun invoke(): SafeFuture<ULong> {
    return getDeploymentBlock()
  }
}

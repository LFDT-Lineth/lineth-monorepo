package linea.contract

import linea.EthLogsSearcher
import linea.contract.events.Upgraded
import linea.domain.BlockParameter
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration

typealias ContractDeploymentBlockNumberProvider = () -> SafeFuture<ULong>

class StaticContractDeploymentBlockNumberProvider(
  private val deploymentBlockNumber: ULong,
) : ContractDeploymentBlockNumberProvider {
  override fun invoke(): SafeFuture<ULong> {
    return SafeFuture.completedFuture(deploymentBlockNumber)
  }
}

class EventBasedContractDeploymentBlockNumberProvider(
  private val ethLogsSearcher: EthLogsSearcher,
  private val contractAddress: String,
  private val ethLogsSearchMaxBlockRange: UInt = 10_000u,
  private val log: Logger = LogManager.getLogger(EventBasedContractDeploymentBlockNumberProvider::class.java),
) : ContractDeploymentBlockNumberProvider {
  private val deploymentBlockNumberCache = AtomicReference<ULong>(0UL)

  fun getDeploymentBlock(): SafeFuture<ULong> {
    if (deploymentBlockNumberCache.get() != 0UL) {
      return SafeFuture.completedFuture(deploymentBlockNumberCache.get())
    }
    // The deployment block is the first block that emitted an Upgraded event. Roll forward in bounded
    // chunks (stopping at the first match) instead of a single getLogs(EARLIEST..LATEST), which
    // rate-limited providers (e.g. Infura) reject for spans > 10_000 blocks. INFINITE timeout so the
    // scan terminates on the first match or on chunk exhaustion, never on the clock. Cached, so this
    // runs at most once.
    return ethLogsSearcher
      .getLogsRollingForward(
        fromBlock = BlockParameter.Tag.EARLIEST,
        toBlock = BlockParameter.Tag.LATEST,
        address = contractAddress,
        topics = listOf(Upgraded.topic),
        chunkSize = ethLogsSearchMaxBlockRange,
        searchTimeout = Duration.INFINITE,
        stopAfterTargetLogsCount = 1u,
      )
      .thenApply { result ->
        val blockNumber = result.logs.minByOrNull { it.blockNumber }?.blockNumber
          ?: throw IllegalStateException("Upgraded event not found: contractAddress=$contractAddress")
        deploymentBlockNumberCache.set(blockNumber)
        blockNumber
      }
      .whenException {
        log.error(
          "Failed to get deployment block number for contract={} errorMessage={}",
          contractAddress,
          it.message,
        )
      }
  }

  override fun invoke(): SafeFuture<ULong> {
    return getDeploymentBlock()
  }
}

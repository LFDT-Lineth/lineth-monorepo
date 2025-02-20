package net.consensys.zkevm.coordinator.app

import build.linea.contract.l1.LineaRollupSmartContractClientReadOnly
import io.vertx.core.Vertx
import linea.domain.BlockParameter
import net.consensys.linea.async.AsyncRetryer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

interface LastFinalizedBlockProvider {
  fun getLastFinalizedBlock(): SafeFuture<ULong>
}

/**
 * This class infers when the last conflation happened based on
 * the last block L1 finalized on L1, and getting L1 block time stamp;
 *
 * It's not a very deterministic/accurate approach, but good enough and avoid managing state in a different database.
 */
class L1BasedLastFinalizedBlockProvider(
  private val vertx: Vertx,
  private val lineaRollupSmartContractClient: LineaRollupSmartContractClientReadOnly,
  private val consistentNumberOfBlocksOnL1: UInt,
  private val numberOfRetries: UInt = Int.MAX_VALUE.toUInt(),
  private val pollingInterval: Duration = 2.seconds
) : LastFinalizedBlockProvider {
  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun getLastFinalizedBlock(): SafeFuture<ULong> {
    val lastObservedBlock = AtomicReference<ULong>(null)
    val numberOfObservations = AtomicInteger(1)
    val isConsistentEnough = { lastPolledBlockNumber: ULong ->
      if (lastPolledBlockNumber == lastObservedBlock.get()) {
        numberOfObservations.incrementAndGet().toUInt() >= consistentNumberOfBlocksOnL1
      } else {
        log.info(
          "Rollup finalized block updated from {} to {}, waiting {} blocks for confirmation",
          lastObservedBlock.get(),
          lastPolledBlockNumber,
          consistentNumberOfBlocksOnL1
        )
        numberOfObservations.set(1)
        lastObservedBlock.set(lastPolledBlockNumber)
        false
      }
    }

    return AsyncRetryer.retry(
      vertx,
      maxRetries = numberOfRetries.toInt(),
      backoffDelay = pollingInterval,
      stopRetriesPredicate = isConsistentEnough
    ) {
      lineaRollupSmartContractClient.finalizedL2BlockNumber(
        blockParameter = BlockParameter.Tag.LATEST
      )
    }
  }
}

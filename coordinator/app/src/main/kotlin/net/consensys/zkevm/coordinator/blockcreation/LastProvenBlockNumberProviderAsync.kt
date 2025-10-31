package net.consensys.zkevm.coordinator.blockcreation

import net.consensys.zkevm.persistence.BatchesRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicLong
import kotlin.math.max

interface LastProvenBlockNumberProviderAsync {
  fun getLastProvenBlockNumber(): SafeFuture<Long>
}

interface LastProvenBlockNumberProviderSync {
  fun getLastKnownProvenBlockNumber(): Long
}

interface LatestL1FinalizedBlockProviderSync {
  fun getLatestL1FinalizedBlock(): Long
}

class BatchesRepoBasedLastProvenBlockNumberProvider(
  startingBlockNumberExclusive: Long,
  latestL1FinalizedBlock: Long,
  private val batchesRepository: BatchesRepository,
) : LastProvenBlockNumberProviderAsync, LastProvenBlockNumberProviderSync, LatestL1FinalizedBlockProviderSync {
  private var latestL1FinalizedBlock: AtomicLong = AtomicLong(latestL1FinalizedBlock)
  private var lastProvenBlock: AtomicLong = AtomicLong(startingBlockNumberExclusive)

  fun updateLatestL1FinalizedBlock(blockNumber: Long): SafeFuture<Unit> {
    latestL1FinalizedBlock.set(blockNumber)
    return SafeFuture.completedFuture(Unit)
  }

  override fun getLastProvenBlockNumber(): SafeFuture<Long> {
    return findAndCacheLastProvenBlockNumberFromDb()
  }

  override fun getLastKnownProvenBlockNumber(): Long {
    return lastProvenBlock.get()
  }

  override fun getLatestL1FinalizedBlock(): Long {
    return latestL1FinalizedBlock.get()
  }

  private fun findAndCacheLastProvenBlockNumberFromDb(): SafeFuture<Long> {
    return batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(
      latestL1FinalizedBlock.get() + 1,
    ).thenApply {
        newValue ->
      if (newValue != null) {
        lastProvenBlock.set(newValue)
        newValue
      } else {
        max(lastProvenBlock.get(), latestL1FinalizedBlock.get())
      }
    }
  }
}

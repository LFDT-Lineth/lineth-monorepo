package net.consensys.zkevm.coordinator.app.conflationbacktesting

import linea.LongRunningService
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture

class ConflationBacktestingService : LongRunningService {
  data class ConflationBacktestingRequest(
    val startBlockNumber: ULong,
    val endBlockNumber: ULong,
  )

  enum class ConflationBacktestingJobStatus {
    IN_PROGRESS,
    COMPLETED,
    FAILED,
  }

  fun submitConflationBacktestingJob(conflationBacktestingRequest: ConflationBacktestingRequest): String {
    return conflationBacktestingRequest.hashCode().toString()
  }

  fun getConflationBacktestingJobStatus(jobId: String): ConflationBacktestingJobStatus {
    return ConflationBacktestingJobStatus.FAILED
  }

  override fun start(): CompletableFuture<Unit> {
    return SafeFuture.completedFuture(Unit)
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.completedFuture(Unit)
  }
}

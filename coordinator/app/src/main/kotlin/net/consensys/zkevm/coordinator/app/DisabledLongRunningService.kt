package net.consensys.zkevm.coordinator.app

import net.consensys.zkevm.LongRunningService
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture

object DisabledLongRunningService : LongRunningService {
  override fun start(): CompletableFuture<Unit> {
    return SafeFuture.completedFuture(Unit)
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.completedFuture(Unit)
  }
}

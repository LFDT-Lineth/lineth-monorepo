package net.consensys.zkevm.ethereum.coordination.blockcreation

import com.github.michaelbull.result.onFailure
import com.github.michaelbull.result.onSuccess
import linea.domain.BlockNumberAndHash
import net.consensys.zkevm.coordinator.clients.RollupForkChoiceUpdatedClient
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class ForkChoiceUpdaterImpl(private val rollupForkChoiceUpdatedClients: List<RollupForkChoiceUpdatedClient>) :
  ForkChoiceUpdater {

  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun updateFinalizedBlock(finalizedBlockNumberAndHash: BlockNumberAndHash): SafeFuture<Void> {
    log.debug(
      "Updating finalized block: {}, to {} clients",
      finalizedBlockNumberAndHash,
      rollupForkChoiceUpdatedClients.size
    )
    val futures: List<SafeFuture<*>> = rollupForkChoiceUpdatedClients.map { rollupForkChoiceUpdatedClient ->
      rollupForkChoiceUpdatedClient
        .rollupForkChoiceUpdated(finalizedBlockNumberAndHash)
        .thenApply { result ->
          result
            .onSuccess {
              if ("success".equals(it.result, true)) {
                log.debug("Result of rollup_ForkChoiceUpdated: {}", it.result)
              } else {
                log.warn("Result of rollup_ForkChoiceUpdated: {}", it.result)
              }
            }.onFailure {
              log.error("Error from rollup_ForkChoiceUpdated: errorMessage={}", it.message)
            }
          Unit
        }
    }
    return SafeFuture.allOf(futures.stream())
  }
}

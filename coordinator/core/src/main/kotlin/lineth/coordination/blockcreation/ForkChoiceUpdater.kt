package lineth.coordination.blockcreation

import lineth.domain.BlockNumberAndHash
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ForkChoiceUpdater {
  fun updateFinalizedBlock(finalizedBlockNumberAndHash: BlockNumberAndHash): SafeFuture<Void>
}

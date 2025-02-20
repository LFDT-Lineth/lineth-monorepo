package net.consensys.zkevm.ethereum.coordination.blockcreation

import linea.domain.BlockNumberAndHash
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ForkChoiceUpdater {
  fun updateFinalizedBlock(finalizedBlockNumberAndHash: BlockNumberAndHash): SafeFuture<Void>
}

package net.consensys.zkevm.ethereum.coordination.blob

import net.consensys.zkevm.domain.BlobRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface BlobCompressionProofHandler {
  fun acceptNewBlobCompressionProof(blobRecord: BlobRecord): SafeFuture<*>
}

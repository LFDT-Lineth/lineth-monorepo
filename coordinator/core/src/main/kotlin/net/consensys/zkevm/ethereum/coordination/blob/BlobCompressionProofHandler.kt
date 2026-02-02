package net.consensys.zkevm.ethereum.coordination.blob

import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.ProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface BlobCompressionProofHandler {
  fun acceptNewBlobCompressionProof(blobRecord: BlobRecord): SafeFuture<*>
}

fun interface BlobCompressionProofRequestHandler {
  fun acceptNewBlobCompressionProofRequest(proofIndex: ProofIndex, unProvenBlobRecord: BlobRecord): SafeFuture<*>
}

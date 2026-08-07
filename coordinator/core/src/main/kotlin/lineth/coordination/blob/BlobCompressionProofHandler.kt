package lineth.coordination.blob

import lineth.domain.BlobRecord
import lineth.domain.CompressionProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface BlobCompressionProofHandler {
  fun acceptNewBlobCompressionProof(blobRecord: BlobRecord): SafeFuture<*>
}

fun interface BlobCompressionProofRequestHandler {
  fun acceptNewBlobCompressionProofRequest(
    proofIndex: CompressionProofIndex,
    unProvenBlobRecord: BlobRecord,
  )
}

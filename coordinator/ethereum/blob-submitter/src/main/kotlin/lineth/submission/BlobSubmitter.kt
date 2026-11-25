package lineth.submission

import linea.domain.BlobRecord
import linea.domain.RiscvRollupRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface BlobSubmitter {
  fun submitBlobs(blobsChunks: List<List<BlobRecord>>): SafeFuture<List<String>>

  fun submitBlobCall(blobRecords: List<BlobRecord>): SafeFuture<*>
}

interface BlobSubmitterV2 {
  fun submitBlobs(blobsChunks: List<RiscvRollupRecord>): SafeFuture<List<String>>

  fun submitBlobCall(blobRecord: RiscvRollupRecord): SafeFuture<*>
}

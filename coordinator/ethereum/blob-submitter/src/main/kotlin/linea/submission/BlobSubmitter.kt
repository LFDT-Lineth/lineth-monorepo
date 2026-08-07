package linea.submission

import lineth.domain.BlobRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface BlobSubmitter {
  fun submitBlobs(blobsChunks: List<List<BlobRecord>>): SafeFuture<List<String>>

  fun submitBlobCall(blobRecords: List<BlobRecord>): SafeFuture<*>
}

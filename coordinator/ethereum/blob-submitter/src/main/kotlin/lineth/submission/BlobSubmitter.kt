package lineth.submission

import linea.domain.BlobRecord
import linea.domain.BlobRecordV2
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface BlobSubmitter {
  fun submitBlobs(blobsChunks: List<List<BlobRecord>>): SafeFuture<List<String>>

  fun submitBlobCall(blobRecords: List<BlobRecord>): SafeFuture<*>
}

interface BlobSubmitterV2 {
  fun submitBlobs(blobsChunks: List<BlobRecordV2>): SafeFuture<List<String>>

  fun submitBlobCall(blobRecord: BlobRecordV2): SafeFuture<*>
}

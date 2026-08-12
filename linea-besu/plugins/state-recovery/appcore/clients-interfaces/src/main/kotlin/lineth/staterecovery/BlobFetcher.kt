package lineth.staterecovery

import tech.pegasys.teku.infrastructure.async.SafeFuture

interface BlobFetcher {
  fun fetchBlobsByHash(blobVersionedHashes: List<ByteArray>): SafeFuture<List<ByteArray>>
}

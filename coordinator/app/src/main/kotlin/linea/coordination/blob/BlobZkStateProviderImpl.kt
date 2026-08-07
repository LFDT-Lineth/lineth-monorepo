package linea.coordination.blob

import lineth.clients.GetStateMerkleProofRequest
import lineth.clients.StateManagerClientV1
import lineth.domain.BlockInterval
import tech.pegasys.teku.infrastructure.async.SafeFuture

class BlobZkStateProviderImpl(
  private val zkStateClient: StateManagerClientV1,
) : BlobZkStateProvider {
  override fun getBlobZKState(blockRange: ULongRange): SafeFuture<BlobZkState> {
    return zkStateClient
      .makeRequest(GetStateMerkleProofRequest(BlockInterval(blockRange.first, blockRange.last)))
      .thenApply {
        BlobZkState(
          parentStateRootHash = it.zkParentStateRootHash,
          finalStateRootHash = it.zkEndStateRootHash,
        )
      }
  }
}

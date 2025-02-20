package net.consensys.zkevm.ethereum.coordination.blob

import build.linea.clients.GetStateMerkleProofRequest
import build.linea.clients.StateManagerClientV1
import linea.domain.BlockInterval
import tech.pegasys.teku.infrastructure.async.SafeFuture

class BlobZkStateProviderImpl(
  private val zkStateClient: StateManagerClientV1
) : BlobZkStateProvider {
  override fun getBlobZKState(blockRange: ULongRange): SafeFuture<BlobZkState> {
    return zkStateClient
      .makeRequest(GetStateMerkleProofRequest(BlockInterval(blockRange.first, blockRange.last)))
      .thenApply {
        BlobZkState(
          parentStateRootHash = it.zkParentStateRootHash,
          finalStateRootHash = it.zkEndStateRootHash
        )
      }
  }
}

package linea.ftx

import net.consensys.zkevm.coordinator.clients.InvalidityProofRequest
import net.consensys.zkevm.coordinator.clients.InvalidityProofResponse
import net.consensys.zkevm.coordinator.clients.InvalidityProverClientV1
import net.consensys.zkevm.domain.InvalidityProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FakeInvalidityProverClient() : InvalidityProverClientV1 {
  override fun requestProof(proofRequest: InvalidityProofRequest): SafeFuture<InvalidityProofResponse> {
    TODO("Not yet implemented")
  }

  override fun findProofResponse(proofIndex: InvalidityProofIndex): SafeFuture<InvalidityProofResponse?> {
    TODO("Not yet implemented")
  }

  override fun createProofRequest(proofRequest: InvalidityProofRequest): SafeFuture<InvalidityProofIndex> {
    TODO("Not yet implemented")
  }
}

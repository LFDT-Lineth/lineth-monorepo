package linea.ftx

import linea.clients.InvalidityProofRequest
import linea.clients.InvalidityProofResponse
import linea.clients.InvalidityProverClientV1
import linea.domain.InvalidityProofIndex
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

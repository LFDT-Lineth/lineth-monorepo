package lineth.ftx

import lineth.clients.InvalidityProofRequest
import lineth.clients.InvalidityProofResponse
import lineth.clients.InvalidityProverClientV1
import lineth.domain.InvalidityProofIndex
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

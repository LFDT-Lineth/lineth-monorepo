package net.consensys.zkevm.coordinator.clients

import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.domain.ProofsToAggregate
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ProverProofResponseChecker<ProofResponse> {
  fun findProofResponse(proofRequestId: ProofIndex): SafeFuture<ProofResponse?>

  fun isProofAlreadyDone(proofRequestId: ProofIndex): SafeFuture<Boolean> =
    findProofResponse(proofRequestId).thenApply { it != null }
}

interface ProverProofRequestCreator<ProofRequest> {
  fun createProofRequest(proofRequest: ProofRequest): SafeFuture<ProofIndex>
}

interface ProverClient<ProofRequest, ProofResponse> :
  ProverProofResponseChecker<ProofResponse>,
  ProverProofRequestCreator<ProofRequest> {
  fun requestProof(proofRequest: ProofRequest): SafeFuture<ProofResponse>
}

typealias BlobCompressionProverClientV2 = ProverClient<BlobCompressionProofRequest, BlobCompressionProof>
typealias ProofAggregationProverClientV2 = ProverClient<ProofsToAggregate, ProofToFinalize>
typealias ExecutionProverClientV2 = ProverClient<BatchExecutionProofRequestV1, BatchExecutionProofResponse>
typealias InvalidityProverClientV1 = ProverClient<InvalidityProofRequest, InvalidityProofResponse>

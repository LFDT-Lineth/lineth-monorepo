package net.consensys.zkevm.coordinator.clients

import net.consensys.zkevm.domain.AggregationProofIndex
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.domain.ExecutionProofIndex
import net.consensys.zkevm.domain.InvalidityProofIndex
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.domain.ProofsToAggregate
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ProverProofResponseChecker<ProofResponse, TProofIndex : ProofIndex> {
  fun findProofResponse(proofIndex: TProofIndex): SafeFuture<ProofResponse?>

  fun isProofAlreadyDone(proofIndex: TProofIndex): SafeFuture<Boolean> =
    findProofResponse(proofIndex).thenApply { it != null }
}

interface ProverProofRequestCreator<ProofRequest : Any, TProofIndex : ProofIndex> {
  fun createProofRequest(proofRequest: ProofRequest): SafeFuture<TProofIndex>
}

interface ProverClient<ProofRequest : Any, ProofResponse, TProofIndex : ProofIndex> :
  ProverProofResponseChecker<ProofResponse, TProofIndex>,
  ProverProofRequestCreator<ProofRequest, TProofIndex> {
  fun requestProof(proofRequest: ProofRequest): SafeFuture<ProofResponse>
}

typealias BlobCompressionProverClientV2 =
  ProverClient<BlobCompressionProofRequest, BlobCompressionProof, CompressionProofIndex>
typealias ProofAggregationProverClientV2 = ProverClient<ProofsToAggregate, ProofToFinalize, AggregationProofIndex>
typealias ExecutionProverClientV2 =
  ProverClient<BatchExecutionProofRequestV1, BatchExecutionProofResponse, ExecutionProofIndex>
typealias InvalidityProverClientV1 = ProverClient<InvalidityProofRequest, InvalidityProofResponse, InvalidityProofIndex>

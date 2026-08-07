package lineth.clients

import lineth.domain.AggregationProofIndex
import lineth.domain.BlobCompressionProof
import lineth.domain.BlobCompressionProofRequest
import lineth.domain.BlockIntervalProofIndex
import lineth.domain.CompressionProofIndex
import lineth.domain.ExecutionProofIndex
import lineth.domain.InvalidityProofIndex
import lineth.domain.ProofIndex
import lineth.domain.ProofToFinalize
import lineth.domain.ProofsToAggregate
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

typealias L2ExecutionProverClientV1 =
  ProverClient<L2ExecutionProofRequestV1, L2ExecutionProofResponseV1, BlockIntervalProofIndex>
typealias RollupProverClientV1 =
  ProverClient<RollupProofRequestV1, RollupProofResponseV1, BlockIntervalProofIndex>
typealias RollupAggregationProverClientV1 =
  ProverClient<RollupAggregationProofRequestV1, RollupAggregationProofResponseV1, BlockIntervalProofIndex>

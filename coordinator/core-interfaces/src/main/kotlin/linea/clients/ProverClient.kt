package linea.clients

import linea.domain.AggregationProofIndex
import linea.domain.BlobCompressionProof
import linea.domain.BlobCompressionProofRequest
import linea.domain.BlockIntervalProofIndex
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.domain.InvalidityProofIndex
import linea.domain.ProofIndex
import linea.domain.ProofToFinalize
import linea.domain.ProofsToAggregate
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ProverProofResponseChecker<ProofResponse, TProofIndex : ProofIndex> {
  fun findProofResponse(proofIndex: TProofIndex): SafeFuture<ProofResponse?>

  fun isProofAlreadyDone(proofIndex: TProofIndex): SafeFuture<Boolean>
}

interface ProverProofRequestCreator<ProofRequest : Any, TProofIndex : ProofIndex> {
  fun getProofIndex(proofRequest: ProofRequest): TProofIndex
  fun createProofRequest(proofRequest: ProofRequest): SafeFuture<TProofIndex>
}

interface ProverProofRequestRemover {
  fun removeRequests(startBlockNumberGte: Long? = null): SafeFuture<Unit>
}

interface ProverClient<ProofRequest : Any, ProofResponse, TProofIndex : ProofIndex> :
  ProverProofResponseChecker<ProofResponse, TProofIndex>,
  ProverProofRequestCreator<ProofRequest, TProofIndex> {
  fun requestProof(proofRequest: ProofRequest): SafeFuture<ProofResponse>
}

interface ProverClientV2<ProofRequest : Any, ProofResponse, TProofIndex : ProofIndex> :
  ProverClient<ProofRequest, ProofResponse, TProofIndex>, ProverProofRequestRemover

typealias BlobCompressionProverClientV2 =
  ProverClientV2<BlobCompressionProofRequest, BlobCompressionProof, CompressionProofIndex>
typealias ProofAggregationProverClientV2 =
  ProverClientV2<ProofsToAggregate, ProofToFinalize, AggregationProofIndex>
typealias ExecutionProverClientV2 =
  ProverClientV2<BatchExecutionProofRequestV1, BatchExecutionProofResponse, ExecutionProofIndex>
typealias InvalidityProverClientV1 =
  ProverClientV2<InvalidityProofRequest, InvalidityProofResponse, InvalidityProofIndex>

typealias L2ExecutionProverClientV1 =
  ProverClientV2<L2ExecutionProofRequestV1, L2ExecutionProofResponseV1, BlockIntervalProofIndex>
typealias RollupProverClientV1 =
  ProverClientV2<RollupProofRequestV1, RollupProofResponseV1, BlockIntervalProofIndex>
typealias RollupAggregationProverClientV1 =
  ProverClientV2<RollupAggregationProofRequestV1, RollupAggregationProofResponseV1, BlockIntervalProofIndex>

package linea.coordinator.clients.prover.riscv

import linea.clients.ProverFileNameProvider
import linea.domain.AggregationProofIndex
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.kotlin.encodeHex

object FileNameSuffixes {
  const val L2_EXECUTION_PROOF_SUFFIX = "getZkL2ExecutionProof.json"
  const val ROLLUP_PROOF_SUFFIX = "getZkRollupProof.json"
  const val ROLLUP_AGGREGATION_PROOF_SUFFIX = "getZkRollupAggregationProof.json"
}

private fun encodeHash(hash: ByteArray): String = hash.encodeHex(prefix = false)

object L2ExecutionProofFileNameProvider : ProverFileNameProvider<ExecutionProofIndex> {
  override fun getFileName(proofIndex: ExecutionProofIndex): String {
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-${FileNameSuffixes.L2_EXECUTION_PROOF_SUFFIX}"
  }
}

object RollupProofFileNameProvider : ProverFileNameProvider<CompressionProofIndex> {
  override fun getFileName(proofIndex: CompressionProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-" +
      "-$requestHashString-${FileNameSuffixes.ROLLUP_PROOF_SUFFIX}"
  }
}

object RollupAggregationProofFileNameProvider : ProverFileNameProvider<AggregationProofIndex> {
  override fun getFileName(proofIndex: AggregationProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)

    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${FileNameSuffixes.ROLLUP_AGGREGATION_PROOF_SUFFIX}"
  }
}

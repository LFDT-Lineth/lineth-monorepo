package linea.coordinator.clients.prover.riscv

import linea.clients.ProverFileNameProvider
import linea.domain.BlockIntervalProofIndex
import linea.kotlin.encodeHex

object FileNameSuffixes {
  const val L2_EXECUTION_PROOF_SUFFIX = "getZkL2ExecutionProof.json"
  const val ROLLUP_PROOF_SUFFIX = "getZkRollupProof.json"
  const val ROLLUP_AGGREGATION_PROOF_SUFFIX = "getZkRollupAggregationProof.json"
}

private fun encodeHash(hash: ByteArray): String = hash.encodeHex(prefix = false)

object L2ExecutionProofFileNameProvider : ProverFileNameProvider<BlockIntervalProofIndex> {
  override fun getFileName(proofIndex: BlockIntervalProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${FileNameSuffixes.L2_EXECUTION_PROOF_SUFFIX}"
  }
}

object RollupProofFileNameProvider : ProverFileNameProvider<BlockIntervalProofIndex> {
  override fun getFileName(proofIndex: BlockIntervalProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${FileNameSuffixes.ROLLUP_PROOF_SUFFIX}"
  }
}

object RollupAggregationProofFileNameProvider : ProverFileNameProvider<BlockIntervalProofIndex> {
  override fun getFileName(proofIndex: BlockIntervalProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${FileNameSuffixes.ROLLUP_AGGREGATION_PROOF_SUFFIX}"
  }
}

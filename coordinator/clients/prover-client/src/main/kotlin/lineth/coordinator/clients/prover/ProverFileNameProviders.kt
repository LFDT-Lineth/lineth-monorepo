package lineth.coordinator.clients.prover

import linea.clients.ProverFileNameProvider
import linea.clients.ProverFileNameStartBlockNumberProvider
import linea.domain.BlockIntervalProofIndex
import linea.kotlin.encodeHex

/**
 * Request file name suffixes. The prover routes each request by its file name: names containing
 * `getZkRollupProofV1` or `getZkRollupAggregationProofV1` go to those pipelines, every other name is an l2-execution
 * request. The rollup and aggregation suffixes therefore carry the spec's method names, `V1` included.
 */
object FileNameSuffixes {
  const val L2_EXECUTION_PROOF_SUFFIX = "getZkL2ExecutionProof.json"
  const val ROLLUP_PROOF_SUFFIX = "getZkRollupProofV1.json"
  const val ROLLUP_AGGREGATION_PROOF_SUFFIX = "getZkRollupAggregationProofV1.json"
}

private fun encodeHash(hash: ByteArray): String = hash.encodeHex(prefix = false)

object L2ExecutionProofFileNameProvider :
  ProverFileNameProvider<BlockIntervalProofIndex>, ProverFileNameStartBlockNumberProvider {
  override fun getFileName(proofIndex: BlockIntervalProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${FileNameSuffixes.L2_EXECUTION_PROOF_SUFFIX}"
  }
}

object RollupProofFileNameProvider :
  ProverFileNameProvider<BlockIntervalProofIndex>, ProverFileNameStartBlockNumberProvider {
  override fun getFileName(proofIndex: BlockIntervalProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${FileNameSuffixes.ROLLUP_PROOF_SUFFIX}"
  }
}

object RollupAggregationProofFileNameProvider :
  ProverFileNameProvider<BlockIntervalProofIndex>, ProverFileNameStartBlockNumberProvider {
  override fun getFileName(proofIndex: BlockIntervalProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${FileNameSuffixes.ROLLUP_AGGREGATION_PROOF_SUFFIX}"
  }
}

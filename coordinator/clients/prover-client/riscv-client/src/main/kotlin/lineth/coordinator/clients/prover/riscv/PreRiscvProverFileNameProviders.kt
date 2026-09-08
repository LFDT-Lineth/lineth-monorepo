package lineth.coordinator.clients.prover.riscv

import linea.clients.ProverFileNameProvider
import linea.domain.AggregationProofIndex
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.domain.InvalidityProofIndex
import linea.kotlin.encodeHex

object PreRiscvFileNameSuffixes {
  const val EXECUTION_PROOF_SUFFIX = "getZkProof.json"
  const val COMPRESSION_PROOF_SUFFIX = "getZkBlobCompressionProof.json"
  const val AGGREGATION_PROOF_SUFFIX = "getZkAggregatedProof.json"
  const val INVALIDITY_PROOF_SUFFIX = "getZkInvalidityProof.json"
}

private fun encodeHash(hash: ByteArray): String = hash.encodeHex(prefix = false)

object ExecutionProofFileNameProvider : ProverFileNameProvider<ExecutionProofIndex> {
  override fun getFileName(proofIndex: ExecutionProofIndex): String {
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-" +
      PreRiscvFileNameSuffixes.EXECUTION_PROOF_SUFFIX
  }
}

object CompressionProofRequestFileNameProvider : ProverFileNameProvider<CompressionProofIndex> {
  private const val HARD_CODED_VERSION = "0.0"

  override fun getFileName(proofIndex: CompressionProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-" +
      "bcv$HARD_CODED_VERSION-" +
      "ccv$HARD_CODED_VERSION-" +
      requestHashString + "-" +
      PreRiscvFileNameSuffixes.COMPRESSION_PROOF_SUFFIX
  }
}

object CompressionProofResponseFileNameProvider : ProverFileNameProvider<CompressionProofIndex> {
  override fun getFileName(proofIndex: CompressionProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-" +
      requestHashString + "-" +
      PreRiscvFileNameSuffixes.COMPRESSION_PROOF_SUFFIX
  }
}

object AggregationProofFileNameProvider : ProverFileNameProvider<AggregationProofIndex> {
  override fun getFileName(proofIndex: AggregationProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)

    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${PreRiscvFileNameSuffixes.AGGREGATION_PROOF_SUFFIX}"
  }
}

object InvalidityProofFileNameProvider : ProverFileNameProvider<InvalidityProofIndex> {
  override fun getFileName(proofIndex: InvalidityProofIndex): String {
    return "${proofIndex.simulatedExecutionBlockNumber}-${proofIndex.ftxNumber}" +
      "-${PreRiscvFileNameSuffixes.INVALIDITY_PROOF_SUFFIX}"
  }
}

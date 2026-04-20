package net.consensys.zkevm.coordinator.clients.prover

import linea.domain.AggregationProofIndex
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.domain.InvalidityProofIndex
import linea.domain.ProofIndex
import linea.kotlin.encodeHex

interface ProverFileNameProvider<TProofIndex : ProofIndex> {
  fun getFileName(proofIndex: TProofIndex): String
}

object FileNameSuffixes {
  const val EXECUTION_PROOF_SUFFIX = "getZkProof.json"
  const val COMPRESSION_PROOF_SUFFIX = "getZkBlobCompressionProof.json"
  const val AGGREGATION_PROOF_SUFFIX = "getZkAggregatedProof.json"
  const val INVALIDITY_PROOF_SUFFIX = "getZkInvalidityProof.json"
}

object ExecutionProofFileNameProvider : ProverFileNameProvider<ExecutionProofIndex> {
  override fun getFileName(proofIndex: ExecutionProofIndex): String {
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-${FileNameSuffixes.EXECUTION_PROOF_SUFFIX}"
  }
}

object CompressionProofRequestFileNameProvider : ProverFileNameProvider<CompressionProofIndex> {
  private fun encodeHash(hash: ByteArray): String = hash.encodeHex(prefix = false)
  private const val HARD_CODED_VERSION = "0.0"

  override fun getFileName(proofIndex: CompressionProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-" +
      "bcv$HARD_CODED_VERSION-" +
      "ccv$HARD_CODED_VERSION-" +
      requestHashString + "-" +
      FileNameSuffixes.COMPRESSION_PROOF_SUFFIX
  }
}

object CompressionProofResponseFileNameProvider : ProverFileNameProvider<CompressionProofIndex> {
  private fun encodeHash(hash: ByteArray): String = hash.encodeHex(prefix = false)

  override fun getFileName(proofIndex: CompressionProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-" +
      requestHashString + "-" +
      FileNameSuffixes.COMPRESSION_PROOF_SUFFIX
  }
}

object AggregationProofFileNameProvider : ProverFileNameProvider<AggregationProofIndex> {
  private fun encodeHash(hash: ByteArray): String = hash.encodeHex(prefix = false)

  override fun getFileName(proofIndex: AggregationProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash)

    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-$requestHashString-${FileNameSuffixes.AGGREGATION_PROOF_SUFFIX}"
  }
}

object InvalidityProofFileNameProvider : ProverFileNameProvider<InvalidityProofIndex> {
  override fun getFileName(proofIndex: InvalidityProofIndex): String {
    return "${proofIndex.simulatedExecutionBlockNumber}-${proofIndex.ftxNumber}" +
      "-${FileNameSuffixes.INVALIDITY_PROOF_SUFFIX}"
  }
}

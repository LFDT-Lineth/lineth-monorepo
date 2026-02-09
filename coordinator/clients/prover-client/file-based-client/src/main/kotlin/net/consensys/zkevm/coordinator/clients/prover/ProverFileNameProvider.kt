package net.consensys.zkevm.coordinator.clients.prover

import linea.kotlin.encodeHex
import net.consensys.zkevm.domain.AggregationProofIndex
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.domain.ExecutionProofIndex
import net.consensys.zkevm.domain.InvalidityProofIndex
import net.consensys.zkevm.domain.ProofIndex

interface ProverFileNameProvider<TProofIndex : ProofIndex> {
  fun getFileName(proofIndex: TProofIndex): String
}

object FileNameSuffixes {
  const val EXECUTION_PROOF_SUFFIX = "getZkProof.json"
  const val COMPRESSION_PROOF_SUFFIX = "getZkBlobCompressionProof.json"
  const val AGGREGATION_PROOF_SUFFIX = "getZkAggregatedProof.json"
  const val INVALIDITY_PROOF_SUFFIX = "getZkInvalidityProof.json"
}

class ExecutionProofRequestFileNameProvider(
  private val tracesVersion: String,
  private val stateManagerVersion: String,
) : ProverFileNameProvider<ExecutionProofIndex> {
  override fun getFileName(proofIndex: ExecutionProofIndex): String {
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-etv$tracesVersion-stv$stateManagerVersion" +
      "-${FileNameSuffixes.EXECUTION_PROOF_SUFFIX}"
  }
}
object ExecutionProofResponseFileNameProvider : ProverFileNameProvider<ExecutionProofIndex> {
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

package net.consensys.zkevm.coordinator.clients.prover

import linea.kotlin.encodeHex
import net.consensys.zkevm.domain.ProofIndex

open class ProverFileNameProvider(protected val fileNameSuffix: String) {
  protected fun encodeHash(hash: ByteArray): String = hash.encodeHex(prefix = false)
  private fun fileName(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
    fileNameSuffix: String,
    hash: ByteArray?
  ) =
    if (hash == null || hash.isEmpty()) {
      "$startBlockNumber-$endBlockNumber-$fileNameSuffix"
    } else {
      val hashString = encodeHash(hash)
      "$startBlockNumber-$endBlockNumber-$hashString-$fileNameSuffix"
    }
  open fun getFileName(proofIndex: ProofIndex): String {
    return fileName(
      startBlockNumber = proofIndex.startBlockNumber,
      endBlockNumber = proofIndex.endBlockNumber,
      hash = proofIndex.hash,
      fileNameSuffix = fileNameSuffix
    )
  }
}

object FileNameSuffixes {
  const val EXECUTION_PROOF_SUFFIX = "getZkProof.json"
  const val COMPRESSION_PROOF_SUFFIX = "getZkBlobCompressionProof.json"
  const val AGGREGATION_PROOF_SUFFIX = "getZkAggregatedProof.json"
}

class ExecutionProofRequestFileNameProvider(
  private val tracesVersion: String,
  private val stateManagerVersion: String
) : ProverFileNameProvider(FileNameSuffixes.EXECUTION_PROOF_SUFFIX) {
  override fun getFileName(proofIndex: ProofIndex): String {
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}" +
      "-etv$tracesVersion-stv$stateManagerVersion" +
      "-$fileNameSuffix"
  }
}
object ExecutionProofResponseFileNameProvider : ProverFileNameProvider(FileNameSuffixes.EXECUTION_PROOF_SUFFIX)

object CompressionProofRequestFileNameProvider : ProverFileNameProvider(FileNameSuffixes.COMPRESSION_PROOF_SUFFIX) {
  private const val HARD_CODED_VERSION = "0.0"
  override fun getFileName(proofIndex: ProofIndex): String {
    val requestHashString = encodeHash(proofIndex.hash!!)
    return "${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}-" +
      "bcv$HARD_CODED_VERSION-" +
      "ccv$HARD_CODED_VERSION-" +
      requestHashString + "-" +
      fileNameSuffix
  }
}
object CompressionProofResponseFileNameProvider : ProverFileNameProvider(FileNameSuffixes.COMPRESSION_PROOF_SUFFIX)

object AggregationProofFileNameProvider : ProverFileNameProvider(FileNameSuffixes.AGGREGATION_PROOF_SUFFIX)

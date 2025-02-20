package net.consensys.zkevm.coordinator.clients.prover

import linea.kotlin.decodeHex
import net.consensys.zkevm.domain.ProofIndex
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.Test

class ProverFileNameProvidersTest {

  @Test
  fun test_getExecutionProof_requestFileName() {
    val executionProofRequestFileNameProvider = ExecutionProofRequestFileNameProvider(
      tracesVersion = "0.1",
      stateManagerVersion = "0.2"
    )
    Assertions.assertEquals(
      "11-17-etv0.1-stv0.2-getZkProof.json",
      executionProofRequestFileNameProvider.getFileName(
        ProofIndex(
          startBlockNumber = 11u,
          endBlockNumber = 17u
        )
      )
    )
  }

  @Test
  fun test_getExecutionProof_responseFileName() {
    Assertions.assertEquals(
      "11-17-getZkProof.json",
      ExecutionProofResponseFileNameProvider.getFileName(
        ProofIndex(
          startBlockNumber = 11u,
          endBlockNumber = 17u
        )
      )
    )
  }

  @Test
  fun test_compressionProof_responseFileName() {
    val fileNameProvider = CompressionProofResponseFileNameProvider
    val hash = "0abcd123".decodeHex()
    Assertions.assertEquals(
      "11-17-0abcd123-getZkBlobCompressionProof.json",
      fileNameProvider.getFileName(
        ProofIndex(
          startBlockNumber = 11u,
          endBlockNumber = 17u,
          hash = hash
        )
      )
    )
  }

  @Test
  fun test_compressionProof_requestFileName() {
    val requestHash = "0abcd123".decodeHex()
    val requestFileName = CompressionProofRequestFileNameProvider.getFileName(
      ProofIndex(
        startBlockNumber = 1uL,
        endBlockNumber = 11uL,
        hash = requestHash
      )
    )

    Assertions.assertEquals(
      "1-11-bcv0.0-ccv0.0-0abcd123-getZkBlobCompressionProof.json",
      requestFileName
    )
  }

  @Test
  fun test_agggregationProof_FileName() {
    val hash = "abcd".decodeHex()
    Assertions.assertEquals(
      "11-27-abcd-getZkAggregatedProof.json",
      AggregationProofFileNameProvider.getFileName(
        ProofIndex(
          startBlockNumber = 11u,
          endBlockNumber = 27u,
          hash = hash
        )
      )
    )
  }
}

package net.consensys.zkevm.coordinator.clients.prover.serialization

import linea.domain.BlockIntervals
import net.consensys.zkevm.coordinator.clients.prover.serialization.JsonSerialization.proofResponseMapperV1
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import java.util.stream.Stream
import kotlin.random.Random

class BlobCompressionProofJsonResponseTest {

  @Test
  fun `should serialize and deserialize proof response`() {
    val proofResponse = BlobCompressionProofJsonResponse(
      compressedData = Random.nextBytes(Random.nextInt(80, 128)),
      conflationOrder = BlockIntervals(10UL, listOf(20UL, 30UL)),
      prevShnarf = Random.nextBytes(32),
      parentStateRootHash = Random.nextBytes(32),
      finalStateRootHash = Random.nextBytes(32),
      parentDataHash = Random.nextBytes(32),
      dataHash = Random.nextBytes(32),
      snarkHash = Random.nextBytes(32),
      expectedX = Random.nextBytes(32),
      expectedY = Random.nextBytes(32),
      expectedShnarf = Random.nextBytes(32),
      decompressionProof = Random.nextBytes(512),
      proverVersion = "mock-0.0.0",
      verifierID = 6789,
      eip4844Enabled = false,
      commitment = ByteArray(0),
      kzgProofSidecar = ByteArray(0),
      kzgProofContract = ByteArray(0)
    )
    val proofResponseJsonString = proofResponse.toJsonString()

    assertThat(proofResponseJsonString).isEqualTo(proofResponseMapperV1.writeValueAsString(proofResponse))
    assertThat(BlobCompressionProofJsonResponse.fromJsonString(proofResponseJsonString))
      .isEqualTo(proofResponse)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressionProofResponseFile")
  fun `should deserialize proof response from file`(filePath: Path) {
    val responseJson = Files.readString(filePath)
    val proofResponse = BlobCompressionProofJsonResponse.fromJsonString(responseJson)
    assertThat(proofResponse).isNotNull
  }

  companion object {
    private const val testdataPath1 = "../../../../testdata/prover/blob-compression/responses"
    private const val testdataPath2 = "../../../../testdata/prover-v2/prover-compression/responses/"

    private fun testFiles(): Array<File> {
      val testFiles1 = File(testdataPath1).listFiles()!!
      val testFiles2 = File(testdataPath2).listFiles()!!
      return testFiles1 + testFiles2
    }

    @JvmStatic
    fun compressionProofResponseFile(): Stream<Arguments> {
      return testFiles()
        .filter { it.isFile && it.name.endsWith(".json") }
        .map { filePath ->
          Arguments.of(filePath.toPath())
        }.stream()
    }
  }
}

package linea.domain.serialization

import linea.domain.BlobMetadataV1
import linea.domain.VersionedMetadata
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.time.Instant

class VersionedMetadataCodecTest {
  private val codec = BlobMetadataSerialization.codec

  private fun sampleV1() = BlobMetadataV1(
    startBlockTimestamp = Instant.fromEpochMilliseconds(1_700_000_000_000L),
    endBlockTimestamp = Instant.fromEpochMilliseconds(1_700_000_123_000L),
    batchesCount = 7,
    blobHash = Random.nextBytes(32),
    expectedShnarf = Random.nextBytes(32),
    prevShnarf = Random.nextBytes(32),
    finalStateRootHash = Random.nextBytes(32),
    compressedData = Random.nextBytes(Random.nextInt(80, 200)),
    commitment = Random.nextBytes(48),
    kzgProofContract = Random.nextBytes(48),
    kzgProofSideCar = Random.nextBytes(48),
    expectedX = Random.nextBytes(32),
    expectedY = Random.nextBytes(32),
    snarkHash = Random.nextBytes(32),
  )

  @Test
  fun `round-trips a V1 payload`() {
    val payload = sampleV1()

    val json = codec.toJson(payload)
    val decoded = codec.fromJson(json)

    assertThat(decoded).isEqualTo(VersionedMetadata(1, payload))
    assertThat(decoded.version).isEqualTo(1)
    assertThat(decoded.metadata).isEqualTo(payload)
  }

  @Test
  fun `envelope has version and metadata, ByteArray as 0x-hex, Instant as ISO-8601`() {
    val json = codec.toJson(sampleV1())

    assertThat(json).contains("\"version\":1")
    assertThat(json).contains("\"metadata\":")
    assertThat(json).contains("\"blobHash\":\"0x")
    // Instant is serialized as an ISO-8601 string
    assertThat(json).contains("\"startBlockTimestamp\":\"")
  }

  @Test
  fun `fromJson dispatches payload class by version`() {
    val decoded = codec.fromJson(codec.toJson(sampleV1()))
    assertThat(decoded.metadata).isInstanceOf(BlobMetadataV1::class.java)
  }

  @Test
  fun `fromJson throws on unknown version`() {
    val json = codec.toJson(sampleV1()).replace("\"version\":1", "\"version\":999")
    assertThatThrownBy { codec.fromJson(json) }
      .isInstanceOf(IllegalArgumentException::class.java)
  }
}

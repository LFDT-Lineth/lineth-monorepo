package linea.domain.serialization

import linea.domain.BlobMetadata
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.time.Instant

class BlobMetadataSerializationTest {
  private fun sample() = BlobMetadata(
    startBlockTimestamp = Instant.fromEpochMilliseconds(1_700_000_000_000L),
    endBlockTimestamp = Instant.fromEpochMilliseconds(1_700_000_123_000L),
    batchesCount = 7u,
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
  fun `round-trips a domain BlobMetadata`() {
    val metadata = sample()

    val decoded = BlobMetadataSerialization.deserialize(BlobMetadataSerialization.serialize(metadata))

    assertThat(decoded).isEqualTo(metadata)
  }

  @Test
  fun `serializes as a versioned envelope, ByteArray as 0x-hex, Instant as ISO-8601`() {
    val json = BlobMetadataSerialization.serialize(sample())

    assertThat(json).contains("\"version\":1")
    assertThat(json).contains("\"metadata\":")
    assertThat(json).contains("\"blobHash\":\"0x")
    // Instant serializes as an ISO-8601 string
    assertThat(json).contains("\"startBlockTimestamp\":\"")
    // version/payload naming must not leak into the domain type used by callers
    assertThat(json).doesNotContain("BlobMetadataV1")
  }

  @Test
  fun `deserialize throws on unknown version`() {
    val json = BlobMetadataSerialization.serialize(sample()).replace("\"version\":1", "\"version\":999")

    assertThatThrownBy { BlobMetadataSerialization.deserialize(json) }
      .isInstanceOf(IllegalArgumentException::class.java)
  }
}

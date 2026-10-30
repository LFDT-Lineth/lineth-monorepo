package linea.da

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class DaStreamPositionTest {
  private val hash = ByteArray(32) { 0x01 }
  private val prevHash = ByteArray(32) { 0x02 }
  private val sharedBlob = ByteArray(DaConstants.BLOB_BYTES)

  @Test
  fun `fresh position needs only the data rolling hash`() {
    val position = DaStreamPosition(hash)

    assertThat(position.tailBytes).isZero()
    assertThat(position.sharedBlob).isNull()
    assertThat(position.prevDataRollingHash).isNull()
  }

  @Test
  fun `position inside a shared blob requires the blob and the previous rolling hash`() {
    val position = DaStreamPosition(hash, tailBytes = 100, sharedBlob = sharedBlob, prevDataRollingHash = prevHash)

    assertThat(
      position,
    ).isEqualTo(position.copy(sharedBlob = sharedBlob.copyOf(), prevDataRollingHash = prevHash.copyOf()))
    assertThatThrownBy { DaStreamPosition(hash, tailBytes = 100) }.isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy {
      DaStreamPosition(hash, tailBytes = 100, sharedBlob = ByteArray(10), prevDataRollingHash = prevHash)
    }
      .isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `rejects invalid hashes, tail bytes and stray shared-blob fields`() {
    assertThatThrownBy { DaStreamPosition(ByteArray(31)) }.isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { DaStreamPosition(hash, tailBytes = -1) }.isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy {
      DaStreamPosition(
        hash,
        tailBytes = DaConstants.BLOB_PAYLOAD_CAPACITY,
        sharedBlob = sharedBlob,
        prevDataRollingHash = prevHash,
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy {
      DaStreamPosition(hash, sharedBlob = sharedBlob)
    }.isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `blob payload capacity matches the rollup spec`() {
    assertThat(DaConstants.BLOB_PAYLOAD_CAPACITY).isEqualTo(130_047)
    assertThat(DaConstants.BLOB_BYTES).isEqualTo(131_072)
  }
}

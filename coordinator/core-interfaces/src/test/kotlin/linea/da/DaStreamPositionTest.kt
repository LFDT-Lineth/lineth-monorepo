package linea.da

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class DaStreamPositionTest {
  private val hash = ByteArray(32) { 0x01 }
  private val prevHash = ByteArray(32) { 0x02 }
  private val sharedBlob = ByteArray(DaConstants.BLOB_BYTES)

  @Test
  fun `fresh position starts the next blob after the data rolling hash`() {
    val position = DaStreamPosition.fresh(hash)

    assertThat(position.sharedBlobCarryOverTailBytes).isZero()
    assertThat(position.sharedBlob).isNull()
    assertThat(position.parentBlobDataRollingHash).isEqualTo(hash)
    assertThatThrownBy { DaStreamPosition(hash, prevHash) }.isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `position inside a shared blob requires the blob`() {
    val position = DaStreamPosition(hash, prevHash, sharedBlobCarryOverTailBytes = 100, sharedBlob = sharedBlob)

    assertThat(position).isEqualTo(
      position.copy(parentBlobDataRollingHash = prevHash.copyOf(), sharedBlob = sharedBlob.copyOf()),
    )
    assertThatThrownBy { DaStreamPosition(hash, prevHash, sharedBlobCarryOverTailBytes = 100) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy {
      DaStreamPosition(
        hash,
        prevHash,
        sharedBlobCarryOverTailBytes = 100,
        sharedBlob = ByteArray(10),
      )
    }
      .isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `rejects invalid hashes, tail bytes and a stray shared blob`() {
    assertThatThrownBy { DaStreamPosition.fresh(ByteArray(31)) }.isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy {
      DaStreamPosition(hash, ByteArray(31), sharedBlobCarryOverTailBytes = 100, sharedBlob = sharedBlob)
    }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { DaStreamPosition(hash, hash, sharedBlobCarryOverTailBytes = -1) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy {
      DaStreamPosition(
        hash,
        prevHash,
        sharedBlobCarryOverTailBytes = DaConstants.BLOB_PAYLOAD_CAPACITY,
        sharedBlob = sharedBlob,
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { DaStreamPosition(hash, hash, sharedBlob = sharedBlob) }
      .isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `blob payload capacity matches the rollup spec`() {
    assertThat(DaConstants.BLOB_PAYLOAD_CAPACITY).isEqualTo(130_047)
    assertThat(DaConstants.BLOB_BYTES).isEqualTo(131_072)
  }
}

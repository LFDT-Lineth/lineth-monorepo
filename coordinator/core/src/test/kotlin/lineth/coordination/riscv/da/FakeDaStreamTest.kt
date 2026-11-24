package lineth.coordination.riscv.da

import lineth.coordination.riscv.rollup.Keccak256DataRollingHashCalculator
import lineth.da.DaCodecVersion
import lineth.da.DaConstants.BLOB_BYTES
import lineth.da.DaConstants.BLOB_PAYLOAD_TERMINAL
import lineth.da.DaConstants.KZG_POINT_BYTES
import lineth.da.DaConstants.SEGMENT_LENGTH_PREFIX_BYTES
import lineth.da.DaStreamContractTest
import lineth.da.DaStreamErrorType
import lineth.da.DaStreamException
import lineth.da.DaStreamPosition
import lineth.da.SealedBlob
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.nio.file.Path

class FakeDaStreamTest : DaStreamContractTest(
  factory = FakeDaStreamFactory(),
  version = DaCodecVersion.FAKE,
) {
  override fun expectBlobLayout(blob: SealedBlob, payload: ByteArray) {
    val bytes = blob.blob
    assertThat(bytes).hasSize(BLOB_BYTES)
    assertThat(bytes.copyOfRange(0, payload.size)).isEqualTo(payload)
    assertThat(bytes[payload.size]).isEqualTo(BLOB_PAYLOAD_TERMINAL)
    assertThat(bytes.copyOfRange(payload.size + 1, BLOB_BYTES)).containsOnly(0)
    assertThat(blob.chunkHash).isEqualTo(FakeDaStream.fakeVersionedHash(bytes))
    assertThat(blob.kzgCommitment).hasSize(KZG_POINT_BYTES).isEqualTo(FakeDaStream.fakeKzgCommitment(bytes))
    assertThat(blob.kzgProof).hasSize(KZG_POINT_BYTES).isEqualTo(FakeDaStream.fakeKzgProof(bytes))
    assertThat(blob.kzgCommitment).isNotEqualTo(blob.kzgProof)
    assertThat(blob.dataRollingHash)
      .isEqualTo(Keccak256DataRollingHashCalculator.fold(blob.prevDataRollingHash, blob.chunkHash))
  }

  @Test
  fun `compress concatenates the block RLPs into the frame`() {
    val rlps = listOf(byteArrayOf(0x01, 0x02), byteArrayOf(0x03), byteArrayOf(0x04, 0x05, 0x06))

    val frame = FakeDaCompressor().compress(rlps)

    assertThat(frame).isEqualTo(rlps.reduce(ByteArray::plus))
  }

  @Test
  fun `compress rejects a conflation without blocks`() {
    assertThatThrownBy { FakeDaCompressor().compress(emptyList()) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INVALID_ARGUMENT)
      }
  }

  @Test
  fun `factories create a fake compressor and open a fake stream at the given position`() {
    val genesis = ByteArray(32) { 0x11 }
    val compressor = FakeDaCompressorFactory().create(DaCodecVersion.FAKE, Path.of("unused"), 1337UL)
    val stream = FakeDaStreamFactory().open(DaCodecVersion.FAKE, DaStreamPosition.fresh(genesis))

    stream.append(compressor.compress(listOf(byteArrayOf(0x01, 0x02))), conflationEndBlock = 1UL)
    val blob = stream.flush()!!

    assertThat(blob.prevDataRollingHash).isEqualTo(genesis)
    assertThat(blob.payloadBytes).isEqualTo((SEGMENT_LENGTH_PREFIX_BYTES + 2).toUInt())
  }
}

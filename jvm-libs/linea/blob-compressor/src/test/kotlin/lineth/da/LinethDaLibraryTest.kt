package lineth.da

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.nio.file.Path

class LinethDaLibraryTest {
  @Test
  fun `the struct layouts have the sizes of the header`() {
    assertThat(NativeLayouts.bytes.byteSize()).isEqualTo(16)
    assertThat(NativeLayouts.compressed.byteSize()).isEqualTo(16)
    assertThat(NativeLayouts.segmentEnd.byteSize()).isEqualTo(16)
    assertThat(NativeLayouts.sealedBlob.byteSize()).isEqualTo(131_280)
    assertThat(NativeLayouts.sealedBlobs.byteSize()).isEqualTo(16)
    assertThat(NativeLayouts.streamPosition.byteSize()).isEqualTo(80)
  }

  @Test
  fun `the fields sit where the C compiler puts them`() {
    assertThat(NativeLayouts.sealedBlob.offsetOf("chunk_hash")).isEqualTo(131_072)
    assertThat(NativeLayouts.sealedBlob.offsetOf("kzg_commitment")).isEqualTo(131_104)
    assertThat(NativeLayouts.sealedBlob.offsetOf("kzg_proof")).isEqualTo(131_152)
    assertThat(NativeLayouts.sealedBlob.offsetOf("prev_data_rolling_hash")).isEqualTo(131_200)
    assertThat(NativeLayouts.sealedBlob.offsetOf("data_rolling_hash")).isEqualTo(131_232)
    assertThat(NativeLayouts.sealedBlob.offsetOf("payload_bytes")).isEqualTo(131_264)
    assertThat(NativeLayouts.sealedBlob.offsetOf("segment_end_count")).isEqualTo(131_268)
    assertThat(NativeLayouts.sealedBlob.offsetOf("segment_ends")).isEqualTo(131_272)
    assertThat(NativeLayouts.segmentEnd.offsetOf("unused_tail_bytes")).isEqualTo(8)
    assertThat(NativeLayouts.streamPosition.offsetOf("shared_blob_carry_over_tail_bytes")).isEqualTo(64)
    assertThat(NativeLayouts.streamPosition.offsetOf("shared_blob")).isEqualTo(72)
  }

  @Test
  fun `the library of the tests has the ABI version of the binding`() {
    ZigTestLibrary.assumeAvailable()

    val library = LinethDaLibrary.load(ZigTestLibrary.path!!)

    assertThat(library.abiVersion()).isEqualTo(LinethDaLibrary.ABI_VERSION)
  }

  @Test
  fun `a library of another ABI version is refused at load`() {
    ZigTestLibrary.assumeAvailable()

    assertThatThrownBy { LinethDaLibrary.load(ZigTestLibrary.path!!, abiVersion = LinethDaLibrary.ABI_VERSION + 1) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INTERNAL)
        assertThat(it).hasMessageContaining("ABI version ${LinethDaLibrary.ABI_VERSION}")
      }
  }

  @Test
  fun `a path that holds no library is refused at load`() {
    assertThatThrownBy { LinethDaLibrary.load(Path.of("/no/such/liblineth_da.so")) }
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(DaStreamErrorType.INTERNAL)
      }
  }
}

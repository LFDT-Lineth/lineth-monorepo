package lineth.da

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach

class ZigDaStreamContractTest : DaStreamContractTest(
  factory = DaStreamFactory { version, position -> ZigTestLibrary.factory.open(version, position) },
  version = DaCodecVersion.DEV,
) {
  @BeforeEach
  fun requireLibrary() = ZigTestLibrary.assumeAvailable()

  override fun expectBlobLayout(blob: SealedBlob, payload: ByteArray) {
    assertThat(blob.payloadBytes).isEqualTo(payload.size.toUInt())
  }
}

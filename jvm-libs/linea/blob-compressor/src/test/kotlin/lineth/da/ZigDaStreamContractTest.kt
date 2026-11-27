package lineth.da

import org.assertj.core.api.Assertions.assertThat

class ZigDaStreamContractTest : DaStreamContractTest(
  factory = ZigDaFactory.bundled,
  version = DaCodecVersion.V1,
) {
  override fun expectBlobLayout(blob: SealedBlob, payload: ByteArray) {
    assertThat(blob.payloadBytes).isEqualTo(payload.size.toUInt())
  }
}

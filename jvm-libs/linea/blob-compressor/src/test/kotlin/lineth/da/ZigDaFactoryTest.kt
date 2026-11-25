package lineth.da

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class ZigDaFactoryTest {
  private val genesis = ByteArray(32)

  private fun expectError(type: DaStreamErrorType, message: String, call: () -> Unit) {
    assertThatThrownBy(call)
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(type)
        assertThat(it).hasMessageContaining(message)
      }
  }

  @Test
  fun `the bundled dictionary is the dictionary of the module`() {
    assertThat(ZigDaFactory.bundledDictionary(DaCodecVersion.V1)).hasSize(64 * 1024)
  }

  @Test
  fun `the bundled library serves a compressor and a stream of its version`() {
    ZigDaFactory.bundled.create(DaCodecVersion.V1, chainId = 59_144UL).use {
      assertThat(it.version).isEqualTo(DaCodecVersion.V1)
    }
    ZigDaFactory.bundled.open(DaCodecVersion.V1, DaStreamPosition.fresh(genesis)).use {
      assertThat(it.version).isEqualTo(DaCodecVersion.V1)
    }
  }

  @Test
  fun `a version without a bundled library is refused`() {
    (DaCodecVersion.entries - DaCodecVersion.V1).forEach { version ->
      expectError(DaStreamErrorType.INVALID_ARGUMENT, "no bundled library for $version") {
        ZigDaFactory.bundled.open(version, DaStreamPosition.fresh(genesis))
      }
    }
  }
}

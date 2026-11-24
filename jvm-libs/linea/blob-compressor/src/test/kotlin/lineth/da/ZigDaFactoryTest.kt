package lineth.da

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.nio.file.Files
import java.nio.file.Path

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
    val dictionary = ZigDaFactory.bundledDictionary(ZigTestLibrary.DICTIONARY)

    assertThat(Files.size(dictionary)).isEqualTo(64L * 1024)
  }

  @Test
  fun `a version without a bundled library is refused`() {
    DaCodecVersion.entries.forEach { version ->
      expectError(DaStreamErrorType.INVALID_ARGUMENT, "no bundled library for $version") {
        ZigDaFactory.bundled.open(version, DaStreamPosition.fresh(genesis))
      }
    }
  }

  @Test
  fun `a library from a path serves DEV alone`() {
    val factory = ZigDaFactory.forLibrary(Path.of("/no/such/liblineth_da.so"))

    expectError(DaStreamErrorType.INVALID_ARGUMENT, "not ${DaCodecVersion.FAKE}") {
      factory.open(DaCodecVersion.FAKE, DaStreamPosition.fresh(genesis))
    }
  }

  @Test
  fun `a library from a path that holds no library fails at its first use`() {
    val factory = ZigDaFactory.forLibrary(Path.of("/no/such/liblineth_da.so"))

    expectError(DaStreamErrorType.INTERNAL, "cannot load the library") {
      factory.open(DaCodecVersion.DEV, DaStreamPosition.fresh(genesis))
    }
  }
}

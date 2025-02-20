package linea

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.net.URI

class URIExtensionsTest {
  @Test
  fun `getPortWithSchemaDefaults`() {
    assertThat(URI.create("http://example.com").getPortWithSchemeDefaults()).isEqualTo(80)
    assertThat(URI.create("https://example.com").getPortWithSchemeDefaults()).isEqualTo(443)
    assertThat(URI.create("http://example.com:8080").getPortWithSchemeDefaults()).isEqualTo(8080)
    assertThat(URI.create("https://example.com:8080").getPortWithSchemeDefaults()).isEqualTo(8080)
    assertThat(URI.create("myschema://example.com:8080").getPortWithSchemeDefaults()).isEqualTo(8080)
    assertThatThrownBy { (URI.create("mySchema://example.com").getPortWithSchemeDefaults()) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("Unsupported scheme: mySchema")
  }
}

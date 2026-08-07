package lineth.logging

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class MasksTest {

  @Test
  fun testEndpointPathMask() {
    val endpoint = "https://mainnet.infura.io/v3/123456"
    val masked = maskEndpointPath(endpoint)
    assertThat(masked).isEqualTo("https://mainnet.infura.io/*********")
  }
}

package lineth.coordinator.config.v2.toml

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatIllegalArgumentException
import org.junit.jupiter.api.Test

class SignerTypeTest {
  @Test
  fun `resolves signer display names case insensitively`() {
    assertThat(SignerConfigToml.SignerType.valueOfIgnoreCase("WeB3J"))
      .isEqualTo(SignerConfigToml.SignerType.WEB3J)
  }

  @Test
  fun `rejects unknown signer display names`() {
    assertThatIllegalArgumentException()
      .isThrownBy { SignerConfigToml.SignerType.valueOfIgnoreCase("unknown") }
      .withMessage("Unknown signer type: unknown")
  }
}

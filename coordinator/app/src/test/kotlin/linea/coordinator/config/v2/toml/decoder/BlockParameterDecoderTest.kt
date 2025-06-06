package linea.coordinator.config.v2.toml.decoder

import linea.coordinator.config.v2.toml.parseConfig
import linea.domain.BlockParameter
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Test

class BlockParameterDecoderTest {
  @Test
  fun `should decode block parameter tag`() {
    data class ConfigTomTag(val blockParameter: BlockParameter.Tag)

    assertThat(parseConfig<ConfigTomTag>("""block-parameter = "latest" """))
      .isEqualTo(ConfigTomTag(BlockParameter.Tag.LATEST))
  }

  @Test
  fun `should decode block parameter tag nullable`() {
    data class ConfigTomTag(val blockParameter: BlockParameter.Tag? = null)

    assertThat(parseConfig<ConfigTomTag>("""block-parameter = "latest" """))
      .isEqualTo(ConfigTomTag(BlockParameter.Tag.LATEST))

    assertThat(parseConfig<ConfigTomTag>(""" """))
      .isEqualTo(ConfigTomTag(null))
  }

  @Test
  fun `should decode block parameter number`() {
    data class ConfigTomTag(val blockParameter: BlockParameter.BlockNumber)

    assertThat(parseConfig<ConfigTomTag>("""block-parameter = 2_000 """))
      .isEqualTo(ConfigTomTag(BlockParameter.BlockNumber(2_000UL)))
  }

  @Disabled("fails with Cannot cast java.lang.Long to linea.domain.BlockParameter.BlockNumber")
  fun `should decode block parameter number nullable`() {
    data class ConfigTomTag(val blockParameter: BlockParameter.BlockNumber? = null)

    assertThat(parseConfig<ConfigTomTag>("""block-parameter = 2_000 """))
      .isEqualTo(ConfigTomTag(BlockParameter.BlockNumber(2_000UL)))

    assertThat(parseConfig<ConfigTomTag>(""" """))
      .isEqualTo(ConfigTomTag(null))
  }

  @Test
  fun `should decode block parameter generic type with tag set`() {
    data class ConfigTomTag(val blockParameter: BlockParameter)

    assertThat(parseConfig<ConfigTomTag>("""block-parameter = "latest" """))
      .isEqualTo(ConfigTomTag(BlockParameter.Tag.LATEST))
  }

  @Test
  fun `should decode block parameter generic type with number set`() {
    data class ConfigTomTag(val blockParameter: BlockParameter)

    assertThat(parseConfig<ConfigTomTag>("""block-parameter = 2_000 """))
      .isEqualTo(ConfigTomTag(BlockParameter.BlockNumber(2_000UL)))
  }
}

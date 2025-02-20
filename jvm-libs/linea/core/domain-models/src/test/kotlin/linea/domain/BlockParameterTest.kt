package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class BlockParameterTest {

  @Test
  fun `parse should parse valid tag`() {
    assertThat(BlockParameter.parse("earliest")).isEqualTo(BlockParameter.Tag.EARLIEST)
    assertThat(BlockParameter.parse("latest")).isEqualTo(BlockParameter.Tag.LATEST)
    assertThat(BlockParameter.parse("lAtEst")).isEqualTo(BlockParameter.Tag.LATEST)
  }

  @Test
  fun `parse should parse valid decimal number`() {
    assertThat(BlockParameter.parse("120")).isEqualTo(BlockParameter.BlockNumber(120UL))
  }

  @Test
  fun `parse should parse valid hexdecimal number`() {
    assertThat(BlockParameter.parse("0x78")).isEqualTo(BlockParameter.BlockNumber(120UL))
  }

  @Test
  fun `parse should throw InvalidArgument when invalid`() {
    assertThatThrownBy { BlockParameter.parse("invalid") }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("Invalid BlockParameter: invalid")
  }
}

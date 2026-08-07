package lineth.jsonrpc.argument

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import java.lang.IllegalArgumentException

class ArgumentGetterTest {

  @Test
  fun throwsExceptionWhenSizeIsLowerThanArgPosition() {
    val ex =
      assertThrows<IllegalArgumentException> {
        getArgument(String::class, listOf("0x00"), 1, "arg1")
      }
    assertThat(ex.message).contains("arg1 not provided")
  }

  @Test
  fun throwsExceptionWhenArgumentIsNullAndNullableIsFalse() {
    val ex =
      assertThrows<IllegalArgumentException> {
        getArgument(String::class, listOf("v0", null, "v2"), 1, "arg1")
      }
    assertThat(ex.message).contains("Required argument arg1")
  }

  @Test
  fun returnNullIfArgumentIsNullAndNullableIsTrue() {
    assertThat(getArgument(String::class, listOf("v0", null, "v2"), 1, "arg1", true)).isNull()
  }

  @Test
  fun returnArgument() {
    val res: Int = getArgument(Int::class, listOf("v0", null, 10), 2, "arg2")
    assertThat(res).isEqualTo(10)
  }
}

package linea.error

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class JsonRpcErrorResponseExceptionTest {
  @Test
  fun `message does not contain a null method prefix`() {
    val exception = JsonRpcErrorResponseException(
      rpcErrorCode = -1,
      rpcErrorMessage = "failure",
    )

    assertThat(exception.message)
      .isEqualTo("code=-1 message=failure errorData=null")
  }

  @Test
  fun `message includes method when present`() {
    val exception = JsonRpcErrorResponseException(
      rpcErrorCode = -1,
      rpcErrorMessage = "failure",
      method = "linea_test",
    )

    assertThat(exception.message)
      .isEqualTo("linea_test failed with JsonRpcError: code=-1 message=failure errorData=null")
  }
}

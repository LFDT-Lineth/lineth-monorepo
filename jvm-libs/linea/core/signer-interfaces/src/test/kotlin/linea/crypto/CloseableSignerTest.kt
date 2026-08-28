package linea.crypto

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture

class CloseableSignerTest {
  @Test
  fun `close action runs once`() {
    var closeCalls = 0
    val signer = FakeSigner().withCloseAction { closeCalls++ }

    signer.close()
    signer.close()

    assertThat(closeCalls).isEqualTo(1)
  }

  private class FakeSigner : Signer<String> {
    override fun publicKey(): ByteArray = byteArrayOf(1)

    override fun sign(bytes: ByteArray): SafeFuture<String> = SafeFuture.completedFuture("signature")
  }
}

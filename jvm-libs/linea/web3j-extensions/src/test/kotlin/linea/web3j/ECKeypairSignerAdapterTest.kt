package linea.web3j

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Hash
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class ECKeypairSignerAdapterTest {
  private val keyPair: ECKeyPair = ECKeyPair.create(BigInteger.ONE)

  private val signer = object : Signer<Secp256k1Signature> {
    override fun publicKey(): ByteArray = Numeric.toBytesPadded(keyPair.publicKey, 64)

    override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
      val signature = keyPair.sign(Hash.sha3(bytes))
      return SafeFuture.completedFuture(Secp256k1Signature(signature.r, signature.s))
    }
  }

  private val adapter = ECKeypairSignerAdapter(signer)

  @Test
  fun `exposes the delegate public key and refuses to expose a private key`() {
    assertThat(adapter.publicKey).isEqualTo(keyPair.publicKey)
    assertThatThrownBy { adapter.privateKey }
      .isInstanceOf(RuntimeException::class.java)
      .hasMessageContaining("Key is managed by delegated Signer")
  }

  @Test
  fun `sign awaits the delegate and returns its r and s`() {
    val message = "raw transaction bytes".toByteArray()
    val signature = adapter.sign(message)
    val expected = keyPair.sign(Hash.sha3(message))
    assertThat(signature.r).isEqualTo(expected.r)
    assertThat(signature.s).isEqualTo(expected.s)
  }
}

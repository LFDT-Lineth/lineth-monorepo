package linea.crypto

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Hash
import org.web3j.utils.Numeric
import java.math.BigInteger

class ECKeypairSignerTest {
  private val keyPair: ECKeyPair = ECKeyPair.create(BigInteger.ONE)
  private val signer = ECKeypairSigner(keyPair)

  @Test
  fun `publicKey returns the 64-byte padded point`() {
    assertThat(signer.publicKey()).isEqualTo(Numeric.toBytesPadded(keyPair.publicKey, 64))
  }

  @Test
  fun `signs the keccak256 digest of the payload`() {
    val payload = "payload to sign".toByteArray()
    val signature = Secp256k1Signature.fromBytes(signer.sign(payload).get())
    val expected = keyPair.sign(Hash.sha3(payload))
    assertThat(signature.r).isEqualTo(expected.r)
    assertThat(signature.s).isEqualTo(expected.s)
  }
}

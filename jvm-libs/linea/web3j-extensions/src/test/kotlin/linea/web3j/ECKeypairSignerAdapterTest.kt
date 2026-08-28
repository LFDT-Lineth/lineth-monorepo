package linea.web3j

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Keys
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.SignedRawTransaction
import org.web3j.crypto.TransactionDecoder
import org.web3j.service.TxSignServiceImpl
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class ECKeypairSignerAdapterTest {
  private val keyPair: ECKeyPair = ECKeyPair.create(BigInteger.ONE)
  private val signedInputs = mutableListOf<ByteArray>()

  private val signer = object : Signer<Secp256k1Signature> {
    override fun publicKey(): ByteArray = Numeric.toBytesPadded(keyPair.publicKey, 64)

    override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
      signedInputs.add(bytes)
      val signature = keyPair.sign(bytes)
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
    val expected = keyPair.sign(message)
    assertThat(signature.r).isEqualTo(expected.r)
    assertThat(signature.s).isEqualTo(expected.s)
  }

  @Test
  fun `stock transaction signer passes one keccak digest to the delegate`() {
    val chainId = 59144L
    val rawTransaction = RawTransaction.createTransaction(
      chainId,
      BigInteger.ZERO,
      BigInteger.valueOf(21_000),
      "0x000000000000000000000000000000000000dead",
      BigInteger.ONE,
      "0x",
      BigInteger.valueOf(1_000_000_000),
      BigInteger.valueOf(10_000_000_000),
    )

    val signed = TxSignServiceImpl(Credentials.create(adapter)).sign(rawTransaction, chainId)
    val decoded = TransactionDecoder.decode(Numeric.toHexString(signed)) as SignedRawTransaction

    assertThat(signedInputs).hasSize(1)
    assertThat(signedInputs.single()).hasSize(32)
    assertThat(decoded.from).isEqualToIgnoringCase("0x${Keys.getAddress(keyPair)}")
  }
}

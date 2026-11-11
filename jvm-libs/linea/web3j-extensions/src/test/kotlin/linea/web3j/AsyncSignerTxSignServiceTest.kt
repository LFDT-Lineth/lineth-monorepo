package linea.web3j

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import linea.domain.Constants
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.crypto.Blob
import org.web3j.crypto.Credentials
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Hash
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.service.TxSignServiceImpl
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class AsyncSignerTxSignServiceTest {
  private val keyPair: ECKeyPair = ECKeyPair.create(BigInteger.ONE)
  private val expectedSigner = TxSignServiceImpl(Credentials.create(keyPair))

  private fun signer(sign: (ByteArray) -> SafeFuture<Secp256k1Signature>) = object : Signer<Secp256k1Signature> {
    override fun publicKey(): ByteArray = Numeric.toBytesPadded(keyPair.publicKey, 64)

    override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> = sign(bytes)
  }

  private fun localSign(digest: ByteArray): Secp256k1Signature =
    keyPair.sign(digest).let { Secp256k1Signature(it.r, it.s) }

  private val eip1559Tx = RawTransaction.createTransaction(
    59144L,
    BigInteger.ZERO,
    BigInteger.valueOf(21_000),
    "0x000000000000000000000000000000000000dead",
    BigInteger.ONE,
    "0x",
    BigInteger.valueOf(1_000_000_000),
    BigInteger.valueOf(10_000_000_000),
  )

  private val legacyTx = RawTransaction.createTransaction(
    BigInteger.ZERO,
    BigInteger.valueOf(1_000_000_000),
    BigInteger.valueOf(21_000),
    "0x000000000000000000000000000000000000dead",
    BigInteger.ONE,
    "0x",
  )

  private val blobTx = RawTransaction.createEip7594Transaction(
    listOf(Blob(ByteArray(Constants.Eip4844BlobSize))),
    59144L,
    BigInteger.ZERO,
    BigInteger.valueOf(1_000_000_000),
    BigInteger.valueOf(10_000_000_000),
    BigInteger.valueOf(21_000),
    "0x000000000000000000000000000000000000dead",
    BigInteger.ZERO,
    "0x",
    BigInteger.valueOf(1_000_000_000),
  )

  @Test
  fun `produces the same signed transactions as web3j local signing`() {
    val txSignService = AsyncSignerTxSignService(signer { SafeFuture.completedFuture(localSign(it)) })

    assertThat(txSignService.address).isEqualTo(expectedSigner.address)
    listOf(eip1559Tx to -1L, blobTx to -1L, legacyTx to -1L, legacyTx to 59144L).forEach { (tx, chainId) ->
      assertThat(txSignService.signAsync(tx, chainId).get()).isEqualTo(expectedSigner.sign(tx, chainId))
      assertThat(txSignService.sign(tx, chainId)).isEqualTo(expectedSigner.sign(tx, chainId))
    }
  }

  @Test
  fun `signAsync does not wait for the signer`() {
    val signature = SafeFuture<Secp256k1Signature>()
    val signed = AsyncSignerTxSignService(signer { signature }).signAsync(eip1559Tx, -1L)
    assertThat(signed).isNotDone()

    signature.complete(localSign(Hash.sha3(TransactionEncoder.encode(eip1559Tx))))
    assertThat(signed.get()).isEqualTo(expectedSigner.sign(eip1559Tx, -1L))
  }
}

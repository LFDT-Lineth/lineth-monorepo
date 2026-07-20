package linea.coordinator.app

import io.vertx.core.Vertx
import linea.coordinator.config.v2.SignerConfig
import linea.coordinator.extensions.CustomSignerFactory
import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.mockito.ArgumentMatchers.any
import org.mockito.ArgumentMatchers.anyString
import org.mockito.Mockito.mock
import org.mockito.Mockito.`when`
import org.web3j.crypto.ECKeyPair
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.response.EthGetTransactionCount
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.CompletableFuture

class CustomSignerTransactionManagerTest {
  private val signerConfig = SignerConfig(
    type = SignerConfig.SignerType.CUSTOM,
    web3j = null,
    web3signer = null,
    custom = SignerConfig.CustomConfig("l1-submitter"),
  )

  @Test
  fun `resolves a custom signer through the supplied factory`() {
    var resolvedName: String? = null

    createTransactionManager(
      vertx = mock(Vertx::class.java),
      signerConfig = signerConfig,
      client = web3jWithZeroNonce(),
      customSignerFactory = CustomSignerFactory { name ->
        resolvedName = name
        TestSigner
      },
    )

    assertThat(resolvedName).isEqualTo("l1-submitter")
  }

  @Test
  fun `fails clearly when custom signer factory is absent`() {
    assertThatThrownBy {
      createTransactionManager(
        vertx = mock(Vertx::class.java),
        signerConfig = signerConfig,
        client = mock(Web3j::class.java),
      )
    }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("l1-submitter")
  }

  private fun web3jWithZeroNonce(): Web3j {
    val web3j = mock(Web3j::class.java)

    @Suppress("UNCHECKED_CAST")
    val request = mock(Request::class.java) as Request<*, EthGetTransactionCount>
    val response = EthGetTransactionCount().apply { result = "0x0" }
    `when`(
      web3j.ethGetTransactionCount(
        anyString(),
        any(DefaultBlockParameter::class.java),
      ),
    ).thenReturn(request)
    `when`(request.method).thenReturn("eth_getTransactionCount")
    `when`(request.sendAsync()).thenReturn(CompletableFuture.completedFuture(response))
    return web3j
  }

  private object TestSigner : Signer<Secp256k1Signature> {
    private val keyPair = ECKeyPair.create(BigInteger.ONE)

    override fun publicKey(): ByteArray = Numeric.toBytesPadded(keyPair.publicKey, 64)

    override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
      val signature = keyPair.sign(bytes)
      return SafeFuture.completedFuture(Secp256k1Signature(signature.r, signature.s))
    }
  }
}

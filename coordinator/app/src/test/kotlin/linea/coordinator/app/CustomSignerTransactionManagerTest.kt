package linea.coordinator.app

import io.vertx.core.Vertx
import linea.anchoring.MessageAnchoringApp
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.MessageAnchoringConfig
import linea.coordinator.config.v2.ProtocolConfig
import linea.coordinator.config.v2.SignerConfig
import linea.coordinator.extensions.CoordinatorExtensionFactory
import linea.coordinator.extensions.CustomSignerFactory
import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import linea.jsonrpc.TestingJsonRpcServer
import net.consensys.linea.async.get
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
import kotlin.time.Duration.Companion.seconds

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

  @Test
  fun `custom signer config requires custom settings`() {
    assertThatThrownBy {
      SignerConfig(
        type = SignerConfig.SignerType.CUSTOM,
        web3j = null,
        web3signer = null,
      )
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("requires custom config")
  }

  @Test
  fun `extension factory has no custom signer by default`() {
    assertThat(CoordinatorExtensionFactory.NOOP.customSignerFactory).isNull()
  }

  @Test
  fun `message anchoring is disabled when its config is absent`() {
    val configs = mock(CoordinatorConfig::class.java)
    `when`(configs.messageAnchoring).thenReturn(null)

    val service = MessageAnchoringAppConfigurator.create(
      vertx = mock(Vertx::class.java),
      configs = configs,
    )

    assertThat(service).isSameAs(DisabledLongRunningService)
  }

  @Test
  fun `message anchoring resolves its custom signer through the supplied factory`() {
    val vertx = Vertx.vertx()
    val jsonRpcServer = TestingJsonRpcServer(vertx = vertx)
    jsonRpcServer.handle("eth_getTransactionCount") { "0x0" }
    jsonRpcServer.handle("eth_chainId") { "0xe708" }
    jsonRpcServer.handle("eth_blockNumber") { "0x0" }
    var resolvedName: String? = null

    try {
      val configs = messageAnchoringConfigs(jsonRpcServer, signerConfig)

      val service = MessageAnchoringAppConfigurator.create(
        vertx = vertx,
        configs = configs,
        customSignerFactory = CustomSignerFactory { name ->
          resolvedName = name
          TestSigner
        },
      )

      assertThat(service).isInstanceOf(MessageAnchoringApp::class.java)
      assertThat(resolvedName).isEqualTo("l1-submitter")
    } finally {
      jsonRpcServer.stopHttpServer().get()
      vertx.close().get()
    }
  }

  private fun messageAnchoringConfigs(
    jsonRpcServer: TestingJsonRpcServer,
    signerConfig: SignerConfig,
  ): CoordinatorConfig {
    val messageAnchoringConfig = MessageAnchoringConfig(
      l1Endpoint = jsonRpcServer.httpEndpoint,
      l2Endpoint = jsonRpcServer.httpEndpoint,
      signer = signerConfig,
    )
    val layer1Config = ProtocolConfig.Layer1Config(
      contractAddress = "0x0000000000000000000000000000000000000001",
      blockTime = 1.seconds,
      contractDeploymentBlockNumber = null,
    )
    val layer2Config = ProtocolConfig.Layer2Config(
      contractAddress = "0x0000000000000000000000000000000000000002",
      contractDeploymentBlockNumber = null,
    )
    val protocolConfig = mock(ProtocolConfig::class.java)
    `when`(protocolConfig.l1).thenReturn(layer1Config)
    `when`(protocolConfig.l2).thenReturn(layer2Config)

    val configs = mock(CoordinatorConfig::class.java)
    `when`(configs.messageAnchoring).thenReturn(messageAnchoringConfig)
    `when`(configs.protocol).thenReturn(protocolConfig)
    `when`(configs.smartContractErrors).thenReturn(emptyMap())
    return configs
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

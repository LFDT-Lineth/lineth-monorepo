package net.consensys.zkevm.coordinator.app

import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.core.net.PfxOptions
import io.vertx.ext.web.client.WebClientOptions
import linea.kotlin.encodeHex
import linea.web3j.SmartContractErrors
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.l1.Web3JLineaRollupSmartContractClient
import net.consensys.linea.httprest.client.VertxHttpRestClient
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.ethereum.crypto.Web3SignerRestClient
import net.consensys.zkevm.ethereum.crypto.Web3SignerTxSignService
import net.consensys.zkevm.ethereum.signing.ECKeypairSignerAdapter
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.service.TxSignServiceImpl
import org.web3j.tx.gas.ContractGasProvider
import org.web3j.utils.Numeric
import java.io.FileInputStream
import java.nio.file.Path
import java.security.KeyStore
import javax.net.ssl.KeyManagerFactory
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory

fun createTransactionManager(
  vertx: Vertx,
  signerConfig: linea.coordinator.config.v2.SignerConfig,
  client: Web3j,
): AsyncFriendlyTransactionManager {
  fun loadKeyAndTrustStoreFromFiles(
    webClientOptions: WebClientOptions,
    clientKeystorePath: Path,
    clientKeystorePassword: String,
    trustStorePath: Path,
    trustStorePassword: String,
  ): WebClientOptions {
    // Load client keystore
    val keyStore = KeyStore.getInstance("PKCS12")
    FileInputStream(clientKeystorePath.toAbsolutePath().toString()).use { input ->
      keyStore.load(input, clientKeystorePassword.toCharArray())
    }

    // Initialize KeyManagerFactory for client certificate
    val keyManagerFactory = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm())
    keyManagerFactory.init(keyStore, clientKeystorePassword.toCharArray())

    // Load truststore
    val trustStore = KeyStore.getInstance("PKCS12")
    FileInputStream(trustStorePath.toAbsolutePath().toString()).use { input ->
      trustStore.load(input, trustStorePassword.toCharArray())
    }

    // Initialize TrustManagerFactory for server certificate
    val trustManagerFactory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
    trustManagerFactory.init(trustStore)

    // Initialize SSLContext
    val sslContext = SSLContext.getInstance("TLS")
    sslContext.init(keyManagerFactory.keyManagers, trustManagerFactory.trustManagers, null)

    return webClientOptions
      .setSsl(true)
      .setTrustAll(false)
      .setPfxKeyCertOptions(
        PfxOptions()
          .setPath(clientKeystorePath.toAbsolutePath().toString())
          .setPassword(clientKeystorePassword),
      )
      .setPfxTrustOptions(
        PfxOptions()
          .setPath(trustStorePath.toAbsolutePath().toString())
          .setPassword(trustStorePassword),
      )
      .setVerifyHost(true)
  }

  val transactionSignService = when (signerConfig.type) {
    linea.coordinator.config.v2.SignerConfig.SignerType.WEB3J -> {
      TxSignServiceImpl(Credentials.create(signerConfig.web3j!!.privateKey.encodeHex()))
    }

    linea.coordinator.config.v2.SignerConfig.SignerType.WEB3SIGNER -> {
      val web3SignerConfig = signerConfig.web3signer!!
      val endpoint = web3SignerConfig.endpoint
      val webClientOptions: WebClientOptions =
        WebClientOptions()
          .setKeepAlive(web3SignerConfig.keepAlive)
          .setProtocolVersion(HttpVersion.HTTP_1_1)
          .setMaxPoolSize(web3SignerConfig.maxPoolSize)
          .setDefaultHost(endpoint.host)
          .setDefaultPort(endpoint.port)
          .also {
            if (signerConfig.web3signer.tls != null) {
              loadKeyAndTrustStoreFromFiles(
                webClientOptions = it,
                clientKeystorePath = signerConfig.web3signer.tls.keyStorePath,
                clientKeystorePassword = signerConfig.web3signer.tls.keyStorePassword.value,
                trustStorePath = signerConfig.web3signer.tls.trustStorePath,
                trustStorePassword = signerConfig.web3signer.tls.trustStorePassword.value,
              )
            }
          }
      val httpRestClient = VertxHttpRestClient(webClientOptions, vertx)
      val signer = Web3SignerRestClient(httpRestClient, signerConfig.web3signer.publicKey.encodeHex())
      val signerAdapter = ECKeypairSignerAdapter(signer, Numeric.toBigInt(signerConfig.web3signer.publicKey))
      val web3SignerCredentials = Credentials.create(signerAdapter)
      Web3SignerTxSignService(web3SignerCredentials)
    }
  }

  return AsyncFriendlyTransactionManager(client, transactionSignService, -1L)
}

fun createLineaRollupContractClient(
  contractAddress: String,
  transactionManager: AsyncFriendlyTransactionManager,
  contractGasProvider: ContractGasProvider,
  web3jClient: Web3j,
  smartContractErrors: SmartContractErrors,
  useEthEstimateGas: Boolean,
): LineaRollupSmartContractClient {
  return Web3JLineaRollupSmartContractClient.load(
    contractAddress = contractAddress,
    web3j = web3jClient,
    transactionManager = transactionManager,
    contractGasProvider = contractGasProvider,
    smartContractErrors = smartContractErrors,
    useEthEstimateGas = useEthEstimateGas,
  )
}

package linea.coordinator.app

import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.core.http.PoolOptions
import io.vertx.core.net.PfxOptions
import io.vertx.ext.web.client.WebClientOptions
import linea.contract.l1.LineaSmartContractClient
import linea.coordinator.config.v2.L1SubmissionConfig
import linea.coordinator.config.v2.SignerConfig
import linea.crypto.Web3SignerRestClient
import linea.ethapi.EthLogsSearcherImpl
import linea.kotlin.encodeHex
import linea.web3j.ECKeypairSignerAdapter
import linea.web3j.SmartContractErrors
import linea.web3j.Web3SignerTxSignService
import linea.web3j.ethapi.createEthApiClient
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.l1.Web3JLineaRollupSmartContractClient
import net.consensys.linea.contract.l1.Web3JLineaValidiumSmartContractClient
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.WMAGasProvider
import net.consensys.linea.httprest.client.VertxHttpRestClient
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.service.TxSignServiceImpl
import org.web3j.tx.gas.ContractGasProvider
import java.io.FileInputStream
import java.nio.file.Path
import java.security.KeyStore
import javax.net.ssl.KeyManagerFactory
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory

fun createTransactionManager(
  vertx: Vertx,
  signerConfig: SignerConfig,
  client: Web3j,
  log: Logger = LogManager.getLogger("clients.web3signer"),
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
    SSLContext.getInstance("TLS")
      .init(keyManagerFactory.keyManagers, trustManagerFactory.trustManagers, null)

    return webClientOptions
      .setSsl(true)
      .setTrustAll(false)
      .setKeyCertOptions(
        PfxOptions()
          .setPath(clientKeystorePath.toAbsolutePath().toString())
          .setPassword(clientKeystorePassword),
      )
      .setTrustOptions(
        PfxOptions()
          .setPath(trustStorePath.toAbsolutePath().toString())
          .setPassword(trustStorePassword),
      )
      .setVerifyHost(true)
  }

  val transactionSignService =
    when (signerConfig.type) {
      SignerConfig.SignerType.WEB3J -> {
        TxSignServiceImpl(Credentials.create(signerConfig.web3j!!.privateKey.encodeHex()))
      }

      SignerConfig.SignerType.WEB3SIGNER -> {
        val web3SignerConfig = signerConfig.web3signer!!
        val endpoint = web3SignerConfig.endpoint
        val webClientOptions: WebClientOptions =
          WebClientOptions()
            .setKeepAlive(web3SignerConfig.keepAlive)
            .setProtocolVersion(HttpVersion.HTTP_1_1)
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
        val poolOptions = PoolOptions()
          .setHttp1MaxSize(web3SignerConfig.maxPoolSize)
        val httpRestClient = VertxHttpRestClient(webClientOptions, poolOptions, vertx, log)
        val signer = Web3SignerRestClient(httpRestClient, signerConfig.web3signer.publicKey)
        val signerAdapter = ECKeypairSignerAdapter(signer)
        val web3SignerCredentials = Credentials.create(signerAdapter)
        Web3SignerTxSignService(web3SignerCredentials)
      }
    }

  return AsyncFriendlyTransactionManager(client, transactionSignService, -1L)
}

/**
 * @param useEthEstimateGas `eth_estimateGas` may revert for multi-blob data-submission txs;
 * disable it if it's the case.
 */
fun createLineaContractClient(
  vertx: Vertx,
  dataAvailabilityType: L1SubmissionConfig.DataAvailability,
  contractAddress: String,
  transactionManager: AsyncFriendlyTransactionManager,
  contractGasProvider: ContractGasProvider,
  web3jClient: Web3j,
  smartContractErrors: SmartContractErrors,
  useEthEstimateGas: Boolean,
): LineaSmartContractClient {
  return when (dataAvailabilityType) {
    L1SubmissionConfig.DataAvailability.ROLLUP ->
      Web3JLineaRollupSmartContractClient.load(
        contractAddress = contractAddress,
        web3j = web3jClient,
        transactionManager = transactionManager,
        contractGasProvider = contractGasProvider,
        smartContractErrors = smartContractErrors,
        ethLogsSearcher = EthLogsSearcherImpl(vertx = vertx, ethApiClient = createEthApiClient(web3jClient)),
        useEthEstimateGas = useEthEstimateGas,
      )

    L1SubmissionConfig.DataAvailability.VALIDIUM ->
      Web3JLineaValidiumSmartContractClient.load(
        contractAddress = contractAddress,
        web3j = web3jClient,
        transactionManager = transactionManager,
        contractGasProvider = contractGasProvider,
        smartContractErrors = smartContractErrors,
        useEthEstimateGas = useEthEstimateGas,
      )
  }
}

/**
 * @param useEthEstimateGas `eth_estimateGas` may revert for multi-blob data-submission txs;
 * disable it if it's the case.
 */
fun createLineaContractClient(
  l1ChainId: ULong,
  contractAddress: String,
  smartContractErrors: SmartContractErrors,
  vertx: Vertx,
  l1Web3jClient: Web3j,
  feesFetcher: FeesFetcher,
  signerConfig: SignerConfig,
  gasConfig: L1SubmissionConfig.GasConfig,
  l1MinPriorityFeeCalculator: FeesCalculator,
  dataAvailabilityType: L1SubmissionConfig.DataAvailability,
  useEthEstimateGas: Boolean = false,
): LineaSmartContractClient {
  val l1PriorityFeeCalculator: FeesCalculator = BoundableFeeCalculator(
    BoundableFeeCalculator.Config(
      feeUpperBound = gasConfig.fallback.priorityFeePerGasUpperBound.toDouble(),
      feeLowerBound = gasConfig.fallback.priorityFeePerGasLowerBound.toDouble(),
      feeMargin = 0.0,
    ),
    l1MinPriorityFeeCalculator,
  )
  // The below gas provider will act as the primary gas provider if L1
  // dynamic gas pricing is disabled and will act as a fallback gas provider
  // if L1 dynamic gas pricing is enabled
  val primaryOrFallbackGasProvider = WMAGasProvider(
    chainId = l1ChainId.toLong(),
    feesFetcher = feesFetcher,
    priorityFeeCalculator = l1PriorityFeeCalculator,
    config = WMAGasProvider.Config(
      gasLimit = gasConfig.gasLimit,
      maxFeePerGasCap = gasConfig.maxFeePerGasCap,
      maxPriorityFeePerGasCap = gasConfig.maxPriorityFeePerGasCap,
      maxFeePerBlobGasCap = gasConfig.maxFeePerBlobGasCap,
    ),
  )
  val transactionManager = createTransactionManager(
    vertx = vertx,
    signerConfig = signerConfig,
    client = l1Web3jClient,
  )
  return createLineaContractClient(
    vertx = vertx,
    dataAvailabilityType = dataAvailabilityType,
    contractAddress = contractAddress,
    transactionManager = transactionManager,
    contractGasProvider = primaryOrFallbackGasProvider,
    web3jClient = l1Web3jClient,
    smartContractErrors = smartContractErrors,
    useEthEstimateGas = useEthEstimateGas,
  )
}

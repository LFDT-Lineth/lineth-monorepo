package net.consensys.zkevm.coordinator.app

import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.ext.web.client.WebClientOptions
import linea.web3j.SmartContractErrors
import net.consensys.linea.contract.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.EIP1559GasProvider
import net.consensys.linea.contract.L2MessageService
import net.consensys.linea.contract.LineaRollupAsyncFriendly
import net.consensys.linea.contract.l1.Web3JLineaRollupSmartContractClient
import net.consensys.linea.contract.l2.L2MessageServiceGasLimitEstimate
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.WMAGasProvider
import net.consensys.linea.httprest.client.VertxHttpRestClient
import net.consensys.zkevm.coordinator.app.config.L1Config
import net.consensys.zkevm.coordinator.app.config.L2Config
import net.consensys.zkevm.coordinator.app.config.SignerConfig
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.ethereum.crypto.Web3SignerRestClient
import net.consensys.zkevm.ethereum.crypto.Web3SignerTxSignService
import net.consensys.zkevm.ethereum.signing.ECKeypairSignerAdapter
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.service.TxSignServiceImpl
import org.web3j.tx.gas.ContractGasProvider
import org.web3j.utils.Numeric
import java.net.URI

fun createTransactionManager(
  vertx: Vertx,
  signerConfig: SignerConfig,
  client: Web3j
): AsyncFriendlyTransactionManager {
  val transactionSignService = when (signerConfig.type) {
    SignerConfig.Type.Web3j -> {
      TxSignServiceImpl(Credentials.create(signerConfig.web3j!!.privateKey.value))
    }

    SignerConfig.Type.Web3Signer -> {
      val web3SignerConfig = signerConfig.web3signer!!
      val endpoint = URI(web3SignerConfig.endpoint)
      val webClientOptions: WebClientOptions =
        WebClientOptions()
          .setKeepAlive(web3SignerConfig.keepAlive)
          .setProtocolVersion(HttpVersion.HTTP_1_1)
          .setMaxPoolSize(web3SignerConfig.maxPoolSize.toInt())
          .setDefaultHost(endpoint.host)
          .setDefaultPort(endpoint.port)
      val httpRestClient = VertxHttpRestClient(webClientOptions, vertx)
      val signer = Web3SignerRestClient(httpRestClient, signerConfig.web3signer.publicKey)
      val signerAdapter = ECKeypairSignerAdapter(signer, Numeric.toBigInt(signerConfig.web3signer.publicKey))
      val web3SignerCredentials = Credentials.create(signerAdapter)
      Web3SignerTxSignService(web3SignerCredentials)
    }
  }

  return AsyncFriendlyTransactionManager(client, transactionSignService, -1L)
}

fun instantiateZkEvmContractClient(
  l1Config: L1Config,
  transactionManager: AsyncFriendlyTransactionManager,
  gasFetcher: FeesFetcher,
  priorityFeeCalculator: FeesCalculator,
  client: Web3j,
  smartContractErrors: SmartContractErrors
): LineaRollupAsyncFriendly {
  return LineaRollupAsyncFriendly.load(
    l1Config.zkEvmContractAddress,
    client,
    transactionManager,
    WMAGasProvider(
      client.ethChainId().send().chainId.toLong(),
      gasFetcher,
      priorityFeeCalculator,
      WMAGasProvider.Config(
        gasLimit = l1Config.gasLimit,
        maxFeePerGasCap = l1Config.maxFeePerGasCap,
        maxPriorityFeePerGasCap = l1Config.maxPriorityFeePerGasCap,
        maxFeePerBlobGasCap = l1Config.maxFeePerBlobGasCap
      )
    ),
    smartContractErrors
  )
}

fun createLineaRollupContractClient(
  l1Config: L1Config,
  transactionManager: AsyncFriendlyTransactionManager,
  contractGasProvider: ContractGasProvider,
  web3jClient: Web3j,
  smartContractErrors: SmartContractErrors,
  useEthEstimateGas: Boolean
): LineaRollupSmartContractClient {
  return Web3JLineaRollupSmartContractClient.load(
    contractAddress = l1Config.zkEvmContractAddress,
    web3j = web3jClient,
    transactionManager = transactionManager,
    contractGasProvider = contractGasProvider,
    smartContractErrors = smartContractErrors,
    useEthEstimateGas = useEthEstimateGas
  )
}

fun instantiateL2MessageServiceContractClient(
  l2Config: L2Config,
  transactionManager: AsyncFriendlyTransactionManager,
  l2Client: Web3j,
  smartContractErrors: SmartContractErrors
): L2MessageService {
  val gasProvider = EIP1559GasProvider(
    l2Client,
    EIP1559GasProvider.Config(
      l2Config.gasLimit,
      l2Config.maxFeePerGasCap,
      l2Config.feeHistoryBlockCount,
      l2Config.feeHistoryRewardPercentile
    )
  )

  return L2MessageServiceGasLimitEstimate.load(
    l2Config.messageServiceAddress,
    l2Client,
    transactionManager,
    gasProvider,
    smartContractErrors
  )
}

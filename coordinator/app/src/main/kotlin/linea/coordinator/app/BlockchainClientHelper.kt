package linea.coordinator.app

import io.vertx.core.Vertx
import linea.contract.l1.LineaSmartContractClient
import linea.coordinator.config.v2.L1SubmissionConfig
import linea.coordinator.config.v2.SignerConfig
import linea.ethapi.EthLogsSearcherImpl
import linea.web3j.ECKeypairSignerAdapter
import linea.web3j.SmartContractErrors
import linea.web3j.ethapi.createEthApiClient
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.l1.Web3JLineaRollupSmartContractClient
import net.consensys.linea.contract.l1.Web3JLineaValidiumSmartContractClient
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.service.TxSignServiceImpl
import org.web3j.tx.gas.ContractGasProvider

fun createTransactionManager(
  vertx: Vertx,
  signerConfig: SignerConfig,
  client: Web3j,
  signerFactory: SignerFactory = DefaultSignerFactory,
): AsyncFriendlyTransactionManager {
  val signer = signerFactory.create(vertx, signerConfig)
  val credentials = Credentials.create(ECKeypairSignerAdapter(signer))
  return AsyncFriendlyTransactionManager(client, TxSignServiceImpl(credentials), -1L)
}

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

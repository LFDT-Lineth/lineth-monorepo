package lineth.contract.l1

<<<<<<<< HEAD:coordinator/clients/smart-contract-client/src/main/kotlin/lineth/contract/l1/LineaValidiumEnhancedWrapper.kt
import linea.contract.ValidiumV1
========
import linea.contract.LinethRollupV6
>>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421)):coordinator/clients/smart-contract-client/src/main/kotlin/net/consensys/linea/contract/l1/LinethRollupEnhancedWrapper.kt
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.Web3JContractAsyncHelper
import org.web3j.abi.datatypes.Function
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.RemoteFunctionCall
import org.web3j.protocol.core.methods.response.TransactionReceipt
import org.web3j.tx.gas.ContractGasProvider
import java.math.BigInteger

<<<<<<<< HEAD:coordinator/clients/smart-contract-client/src/main/kotlin/lineth/contract/l1/LineaValidiumEnhancedWrapper.kt
internal class LineaValidiumEnhancedWrapper(
========
internal class LinethRollupEnhancedWrapper(
>>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421)):coordinator/clients/smart-contract-client/src/main/kotlin/net/consensys/linea/contract/l1/LinethRollupEnhancedWrapper.kt
  contractAddress: String,
  web3j: Web3j,
  transactionManager: AsyncFriendlyTransactionManager,
  contractGasProvider: ContractGasProvider,
  private val web3jContractHelper: Web3JContractAsyncHelper,
<<<<<<<< HEAD:coordinator/clients/smart-contract-client/src/main/kotlin/lineth/contract/l1/LineaValidiumEnhancedWrapper.kt
) : ValidiumV1(
========
) : LinethRollupV6(
>>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421)):coordinator/clients/smart-contract-client/src/main/kotlin/net/consensys/linea/contract/l1/LinethRollupEnhancedWrapper.kt
  contractAddress,
  web3j,
  transactionManager,
  contractGasProvider,
) {
  @Synchronized
  override fun executeRemoteCallTransaction(
    function: Function,
    weiValue: BigInteger,
  ): RemoteFunctionCall<TransactionReceipt> = web3jContractHelper.executeRemoteCallTransaction(function, weiValue)

  @Synchronized
  override fun executeRemoteCallTransaction(function: Function): RemoteFunctionCall<TransactionReceipt> =
    web3jContractHelper.executeRemoteCallTransaction(
      function,
      BigInteger.ZERO,
    )
}

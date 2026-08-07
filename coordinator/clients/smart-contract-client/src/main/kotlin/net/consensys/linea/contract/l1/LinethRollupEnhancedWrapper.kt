package net.consensys.linea.contract.l1

import lineth.contract.LinethRollupV6
import lineth.contract.Web3JContractAsyncHelper
import lineth.web3j.transactionmanager.AsyncFriendlyTransactionManager
import org.web3j.abi.datatypes.Function
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.RemoteFunctionCall
import org.web3j.protocol.core.methods.response.TransactionReceipt
import org.web3j.tx.gas.ContractGasProvider
import java.math.BigInteger

internal class LinethRollupEnhancedWrapper(
  contractAddress: String,
  web3j: Web3j,
  transactionManager: AsyncFriendlyTransactionManager,
  contractGasProvider: ContractGasProvider,
  private val web3jContractHelper: Web3JContractAsyncHelper,
) : LinethRollupV6(
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

package net.consensys.linea.contract.l1

import build.linea.contract.LineaRollupV6
import net.consensys.linea.contract.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.Web3JContractAsyncHelper
import org.web3j.abi.datatypes.Function
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.RemoteFunctionCall
import org.web3j.protocol.core.methods.response.TransactionReceipt
import org.web3j.tx.gas.ContractGasProvider
import java.math.BigInteger

internal class LineaRollupEnhancedWrapper(
  contractAddress: String,
  web3j: Web3j,
  asyncTransactionManager: AsyncFriendlyTransactionManager,
  contractGasProvider: ContractGasProvider,
  val helper: Web3JContractAsyncHelper
) : LineaRollupV6(
  contractAddress,
  web3j,
  asyncTransactionManager,
  contractGasProvider
) {
  @Synchronized
  override fun executeRemoteCallTransaction(
    function: Function,
    weiValue: BigInteger
  ): RemoteFunctionCall<TransactionReceipt> = helper.executeRemoteCallTransaction(function, weiValue)

  @Synchronized
  override fun executeRemoteCallTransaction(
    function: Function
  ): RemoteFunctionCall<TransactionReceipt> = helper.executeRemoteCallTransaction(function, BigInteger.ZERO)
}

package linea.web3j

import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.DefaultBlockParameterName

fun Web3j.getTransactionCount(
  address: String,
  blockParameter: DefaultBlockParameter = DefaultBlockParameterName.PENDING
): Int {
  return this.ethGetTransactionCount(address, blockParameter).send().transactionCount.toInt()
}

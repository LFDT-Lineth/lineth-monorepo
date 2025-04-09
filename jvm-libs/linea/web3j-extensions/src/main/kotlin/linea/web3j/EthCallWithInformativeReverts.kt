package linea.web3j

import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.Response
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.core.methods.response.EthCall
import org.web3j.tx.TransactionManager
import org.web3j.tx.exceptions.ContractCallException
import tech.pegasys.teku.infrastructure.async.SafeFuture

typealias SmartContractErrors = Map<String, String>

fun getRevertReason(
  error: Response.Error?,
  smartContractErrors: SmartContractErrors
): String? {
  val errorDataString = error?.data ?: ""
  return if (errorDataString.length > 11) {
    // execution client can return empty data: "0x", so we need to check the length
    val revertId = errorDataString.substring(3, 11).lowercase()
    smartContractErrors[revertId]
  } else {
    "UNKNOWN"
  }
}

private fun getErrorMessage(
  ethCall: EthCall,
  smartContractErrors: SmartContractErrors
): String {
  val revertReason = getRevertReason(ethCall.error, smartContractErrors)

  return String.format(TransactionManager.REVERT_ERR_STR, ethCall.revertReason) +
    " revertReason=$revertReason errorData=${ethCall.error?.data}"
}

fun Web3j.informativeEthCall(
  tx: Transaction,
  smartContractErrors: SmartContractErrors
): SafeFuture<String?> {
  return SafeFuture
    .of(this.ethCall(tx, DefaultBlockParameterName.LATEST).sendAsync())
    .thenApply { ethCall ->
      if (ethCall.isReverted) {
        throw ContractCallException(getErrorMessage(ethCall, smartContractErrors))
      } else {
        ethCall.value
      }
    }
}

package net.consensys.linea.contract

import linea.web3j.padBlobForEip4844Submission
import org.web3j.crypto.Blob
import org.web3j.protocol.core.Response
import org.web3j.protocol.exceptions.TransactionException

internal fun <T> throwExceptionIfJsonRpcErrorReturned(rpcMethod: String, response: Response<T>) {
  if (response.hasError()) {
    val rpcError = response.error
    var errorMessage =
      "$rpcMethod failed with JsonRpcError: code=${rpcError.code} message=${rpcError.message}"
    if (rpcError.data != null) {
      errorMessage += " data=${rpcError.data}"
    }

    throw TransactionException(errorMessage)
  }
}

internal fun List<ByteArray>.toWeb3JTxBlob(): List<Blob> {
  return this.map { Blob(padBlobForEip4844Submission(it)) }
}

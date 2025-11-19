package linea.web3j.mappers

import linea.domain.TransactionForEthCall
import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger

fun TransactionForEthCall.toWeb3j(): org.web3j.protocol.core.methods.request.Transaction {
  // TODO: update once web3j supports these fields
  require(this.accessList == null) { "Web3j eth_call doesn't support accessList" }
  require(this.maxFeePerBlobGas == null) { "Web3j eth_call doesn't support maxFeePerBlobGas" }
  require(this.blobVersionedHashes == null) { "Web3j eth_call doesn't support blobVersionedHashes" }

  return org.web3j.protocol.core.methods.request.Transaction(
    /* from = */
    this.from.encodeHex(),
    /* nonce = */
    this.nonce?.toBigInteger(),
    /* gasPrice = */
    this.gasPrice?.toBigInteger(),
    /* gasLimit = */
    this.gas?.toBigInteger(),
    /* to = */
    this.to?.encodeHex(),
    /* value = */
    this.value,
    /* data = */
    this.data?.encodeHex(),
    /* chainId = */
    null,
    /* maxPriorityFeePerGas = */
    this.maxPriorityFeePerGas?.toBigInteger(),
    /* maxFeePerGas = */
    this.maxFeePerGas?.toBigInteger(),
  )
}

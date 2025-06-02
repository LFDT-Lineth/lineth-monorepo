package linea.domain

import linea.kotlin.eth
import linea.kotlin.gwei
import linea.kotlin.toBigInteger
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.crypto.KeyPair
import org.hyperledger.besu.crypto.SECP256K1
import org.hyperledger.besu.crypto.SECPSignature
import org.hyperledger.besu.crypto.SignatureAlgorithm
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Wei
import java.math.BigInteger
import kotlin.random.Random

object TransactionFactory {
  val secP256K1: SignatureAlgorithm = SECP256K1()
  val defaltSecp256k1: KeyPair = secP256K1.generateKeyPair()

  fun createTransactionFrontier(
    nonce: ULong = 0UL,
    gasLimit: ULong = 21_000UL,
    to: ByteArray? = Random.nextBytes(20), // Nullable for contract creation transactions
    value: BigInteger = 1UL.eth.toBigInteger(),
    input: ByteArray = ByteArray(0),
    r: BigInteger? = null,
    s: BigInteger? = null,
    v: ULong? = null,
    chainId: ULong? = null, // Optional field for EIP-155 transactions
    gasPrice: ULong? = 3UL.gwei, // null for EIP-1559 transactions
    accessList: List<AccessListEntry>? = null, // null non for EIP-2930 transactions
  ): Transaction {
    return createTransaction(
      type = TransactionType.FRONTIER,
      nonce = nonce,
      gasLimit = gasLimit,
      to = to,
      value = value,
      input = input,
      r = r,
      s = s,
      v = v,
      yParity = null,
      chainId = chainId,
      gasPrice = gasPrice,
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
      accessList = accessList,
    )
  }

  fun createTransactionEip1559(
    nonce: ULong = 0UL,
    gasLimit: ULong = 21_000UL,
    to: ByteArray? = Random.nextBytes(20), // Nullable for contract creation transactions
    value: BigInteger = 1UL.eth.toBigInteger(),
    input: ByteArray = ByteArray(0),
    r: BigInteger? = null,
    s: BigInteger? = null,
    yParity: ULong? = null,
    chainId: ULong = 1337UL, // Optional field for EIP-155 transactions
    maxFeePerGas: ULong? = 3UL.gwei, // null for EIP-1559 transactions
    maxPriorityFeePerGas: ULong? = 2UL.gwei, // null for non EIP-1559 transactions
    accessList: List<AccessListEntry>? = null, // null non for EIP-2930 transactions
  ): Transaction = createTransaction(
    type = TransactionType.EIP1559,
    nonce = nonce,
    gasLimit = gasLimit,
    to = to,
    value = value,
    input = input,
    r = r,
    s = s,
    v = yParity,
    yParity = yParity,
    chainId = chainId,
    gasPrice = null,
    maxFeePerGas = maxFeePerGas,
    maxPriorityFeePerGas = maxPriorityFeePerGas,
    accessList = accessList,
  )

  fun createTransaction(
    type: TransactionType = TransactionType.EIP1559,
    nonce: ULong = 0UL,
    gasLimit: ULong = 21_000UL,
    to: ByteArray? = Random.nextBytes(20), // Nullable for contract creation transactions
    value: BigInteger = 1UL.eth.toBigInteger(),
    input: ByteArray = ByteArray(0),
    r: BigInteger? = null,
    s: BigInteger? = null,
    v: ULong? = null,
    yParity: ULong? = null,
    chainId: ULong? = null, // Optional field for EIP-155 transactions
    gasPrice: ULong? = null, // null for EIP-1559 transactions
    maxFeePerGas: ULong? = 3UL.gwei, // null for EIP-1559 transactions
    maxPriorityFeePerGas: ULong? = 2UL.gwei, // null for non EIP-1559 transactions
    accessList: List<AccessListEntry>? = null, // null non for EIP-2930 transactions
  ): Transaction {
    val signatureArgs = listOfNotNull(r, s, v)
    require(signatureArgs.let { it.size == 3 || it.isEmpty() }) {
      "Either all of r, s, and v must be null or all of them must be non-null"
    }
    val eR: BigInteger
    val eS: BigInteger
    val eV: ULong?
    val eyParity: ULong?
    if (signatureArgs.isEmpty()) {
      val sig = computeSignature(
        type = type,
        nonce = nonce,
        gasLimit = gasLimit,
        to = to,
        value = value,
        input = input,
        chainId = chainId,
        gasPrice = gasPrice,
        maxFeePerGas = maxFeePerGas,
        maxPriorityFeePerGas = maxPriorityFeePerGas,
        accessList = accessList,
      )
      eR = sig.r
      eS = sig.s
      eyParity = sig.recId.toULong()
      eV = calcV(type, sig, chainId) ?: eyParity
    } else {
      eR = r!!
      eS = s!!
      eV = v!!
      eyParity = yParity!!
    }

    return Transaction(
      type = type,
      nonce = nonce,
      gasLimit = gasLimit,
      to = to,
      value = value,
      input = input,
      r = eR,
      s = eS,
      v = eV,
      yParity = eyParity,
      chainId = chainId,
      gasPrice = gasPrice,
      maxFeePerGas = maxFeePerGas,
      maxPriorityFeePerGas = maxPriorityFeePerGas,
      accessList = accessList,
    )
  }

  fun Transaction.computeSignature(
    keyPair: KeyPair = defaltSecp256k1,
  ): SECPSignature {
    return computeSignature(
      type = type,
      nonce = nonce,
      gasLimit = gasLimit,
      to = to,
      value = value,
      input = input,
      chainId = chainId,
      gasPrice = gasPrice,
      maxFeePerGas = maxFeePerGas,
      maxPriorityFeePerGas = maxPriorityFeePerGas,
      accessList = accessList,
      keyPair = keyPair,
    )
  }

  fun computeSignature(
    type: TransactionType,
    nonce: ULong,
    gasLimit: ULong,
    to: ByteArray?,
    value: BigInteger,
    input: ByteArray,
    chainId: ULong?,
    gasPrice: ULong?,
    maxFeePerGas: ULong?,
    maxPriorityFeePerGas: ULong?,
    accessList: List<AccessListEntry>?,
    keyPair: KeyPair = defaltSecp256k1,
  ): SECPSignature {
    val besuType = type.toBesu()
    return org.hyperledger.besu.ethereum.core.Transaction.builder()
      .type(besuType)
      .nonce(nonce.toLong())
      .apply { gasPrice?.let { gasPrice(it.toWei()) } }
      .gasLimit(gasLimit.toLong())
      .to(to?.let { Address.wrap(Bytes.wrap(it)) })
      .value(value.toWei())
      .payload(Bytes.wrap(input))
      .apply { chainId?.let { chainId(it.toBigInteger()) } }
      .maxPriorityFeePerGas(maxPriorityFeePerGas?.toWei())
      .maxFeePerGas(maxFeePerGas?.toWei())
      .apply {
        if (besuType.supportsAccessList()) {
          val accList = accessList?.map { entry ->
            org.hyperledger.besu.datatypes.AccessListEntry(
              Address.wrap(Bytes.wrap(entry.address)),
              entry.storageKeys.map { Bytes32.wrap(it) },
            )
          } ?: emptyList()
          accessList(accList)
        }
      }
      .signAndBuild(keyPair)
      .signature
  }

  fun calcV(
    transactionType: TransactionType,
    signature: SECPSignature,
    chainId: ULong?,
  ): ULong? {
    if (transactionType != TransactionType.FRONTIER) {
      // EIP-2718 typed transaction, use yParity:
      return null
    } else {
      val recId = signature.getRecId().toULong()
      return chainId
        ?.let { (recId + 35UL) + (2UL * chainId) }
        ?: (recId + 27UL)
    }
  }

  fun ULong.toWei(): Wei = Wei.of(this.toBigInteger())
  fun BigInteger.toWei(): Wei = Wei.of(this)
  fun TransactionType.toBesu(): org.hyperledger.besu.datatypes.TransactionType {
    return when (this) {
      linea.domain.TransactionType.FRONTIER -> org.hyperledger.besu.datatypes.TransactionType.FRONTIER
      linea.domain.TransactionType.EIP1559 -> org.hyperledger.besu.datatypes.TransactionType.EIP1559
      linea.domain.TransactionType.ACCESS_LIST -> org.hyperledger.besu.datatypes.TransactionType.ACCESS_LIST
      linea.domain.TransactionType.BLOB -> org.hyperledger.besu.datatypes.TransactionType.BLOB
      linea.domain.TransactionType.DELEGATE_CODE -> org.hyperledger.besu.datatypes.TransactionType.DELEGATE_CODE
    }
  }
}

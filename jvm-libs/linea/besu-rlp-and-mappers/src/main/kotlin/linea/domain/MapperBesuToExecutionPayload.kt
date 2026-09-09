package linea.domain

import org.hyperledger.besu.ethereum.core.Block
import org.hyperledger.besu.ethereum.core.encoding.EncodingContext
import org.hyperledger.besu.ethereum.core.encoding.TransactionEncoder
import java.math.BigInteger

fun Block.toExecutionPayload(blockAccessList: ByteArray): ExecutionPayload = ExecutionPayload(
  parentHash = header.parentHash.bytes.toArray(),
  feeRecipient = header.coinbase.bytes.toArray(),
  stateRoot = header.stateRoot.bytes.toArray(),
  receiptsRoot = header.receiptsRoot.bytes.toArray(),
  logsBloom = header.logsBloom.bytes.toArray(),
  prevRandao = header.mixHashOrPrevRandao.toArray(),
  blockNumber = header.number.toULong(),
  gasLimit = header.gasLimit.toULong(),
  gasUsed = header.gasUsed.toULong(),
  timestamp = header.timestamp.toULong(),
  extraData = header.extraData.toArray(),
  baseFeePerGas = header.baseFee.map { it.toBigInteger() }.orElse(BigInteger.ZERO),
  blockHash = header.hash.bytes.toArray(),
  transactions = body.transactions.map {
    TransactionEncoder.encodeOpaqueBytes(it, EncodingContext.BLOCK_BODY).toArray()
  },
  withdrawals = body.withdrawals.orElse(emptyList()).map {
    Withdrawal(
      index = it.index.toLong().toULong(),
      validatorIndex = it.validatorIndex.toLong().toULong(),
      address = it.address.bytes.toArray(),
      amount = it.amount.toLong().toULong(),
    )
  },
  blobGasUsed = header.blobGasUsed.orElse(0L).toULong(),
  excessBlobGas = header.excessBlobGas.map { it.toLong().toULong() }.orElse(0UL),
  blockAccessList = blockAccessList,
  slotNumber = header.optionalSlotNumber.orElse(null)?.toULong(),
)

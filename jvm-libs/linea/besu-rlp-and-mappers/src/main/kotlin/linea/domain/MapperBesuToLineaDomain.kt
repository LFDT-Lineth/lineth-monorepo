package linea.domain

import linea.domain.MapperBesuToLineaDomain.mapToDomain
import net.consensys.toULong
import org.hyperledger.besu.ethereum.core.Transaction
import kotlin.jvm.optionals.getOrNull

fun org.hyperledger.besu.ethereum.core.Block.toDomain(): Block {
  return mapToDomain(this)
}

object MapperBesuToLineaDomain {
  fun mapToDomain(besuBlock: org.hyperledger.besu.ethereum.core.Block): Block {
    val block = Block(
      number = besuBlock.header.getNumber().toULong(),
      hash = besuBlock.header.hash.toArray(),
      parentHash = besuBlock.header.parentHash.toArray(),
      ommersHash = besuBlock.header.ommersHash.toArray(),
      miner = besuBlock.header.coinbase.toArray(),
      stateRoot = besuBlock.header.stateRoot.toArray(),
      transactionsRoot = besuBlock.header.transactionsRoot.toArray(),
      receiptsRoot = besuBlock.header.receiptsRoot.toArray(),
      logsBloom = besuBlock.header.logsBloom.toArray(),
      difficulty = besuBlock.header.difficulty.toBigInteger().toULong(),
      gasLimit = besuBlock.header.gasLimit.toULong(),
      gasUsed = besuBlock.header.gasUsed.toULong(),
      timestamp = besuBlock.header.timestamp.toULong(),
      extraData = besuBlock.header.extraData.toArray(),
      mixHash = besuBlock.header.mixHash.toArray(),
      nonce = besuBlock.header.nonce.toULong(),
      baseFeePerGas = besuBlock.header.baseFee.getOrNull()?.toBigInteger()?.toULong(),
      ommers = besuBlock.body.ommers.map { it.hash.toArray() },
      transactions = besuBlock.body.transactions.map(MapperBesuToLineaDomain::mapToDomain)
    )

    return block
  }

  fun mapToDomain(transaction: Transaction): linea.domain.Transaction {
    return Transaction(
      nonce = transaction.nonce.toULong(),
      gasPrice = transaction.getGasPrice().getOrNull()?.toBigInteger()?.toULong(),
      gasLimit = transaction.gasLimit.toULong(),
      to = transaction.to.getOrNull()?.toArray(),
      value = transaction.value.toBigInteger(),
      input = transaction.payload.toArray(),
      r = transaction.signature.getR(),
      s = transaction.signature.getS(),
      v = transaction.getV().toULong(),
      yParity = transaction.yParity?.toULong(),
      type = transaction.type.toDomain(),
      chainId = transaction.chainId.getOrNull()?.toULong(),
      maxFeePerGas = transaction.maxFeePerGas.getOrNull()?.toBigInteger()?.toULong(),
      maxPriorityFeePerGas = transaction.maxPriorityFeePerGas.getOrNull()?.toBigInteger()?.toULong(),
      accessList = transaction.accessList.getOrNull()?.map { accessListEntry ->
        AccessListEntry(
          accessListEntry.address.toArray(),
          accessListEntry.storageKeys.map { it.toArray() }
        )
      }
    )
  }
}

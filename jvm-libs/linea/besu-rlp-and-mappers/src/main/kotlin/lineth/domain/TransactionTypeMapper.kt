package lineth.domain

import org.hyperledger.besu.datatypes.TransactionType

fun TransactionType.toDomain(): lineth.domain.TransactionType {
  return when (this) {
    TransactionType.FRONTIER -> lineth.domain.TransactionType.FRONTIER
    TransactionType.EIP1559 -> lineth.domain.TransactionType.EIP1559
    TransactionType.ACCESS_LIST -> lineth.domain.TransactionType.ACCESS_LIST
    TransactionType.BLOB -> lineth.domain.TransactionType.BLOB
    TransactionType.DELEGATE_CODE -> lineth.domain.TransactionType.DELEGATE_CODE
  }
}

fun lineth.domain.TransactionType.toBesu(): TransactionType {
  return when (this) {
    lineth.domain.TransactionType.FRONTIER -> TransactionType.FRONTIER
    lineth.domain.TransactionType.EIP1559 -> TransactionType.EIP1559
    lineth.domain.TransactionType.ACCESS_LIST -> TransactionType.ACCESS_LIST
    lineth.domain.TransactionType.BLOB -> TransactionType.BLOB
    lineth.domain.TransactionType.DELEGATE_CODE -> TransactionType.DELEGATE_CODE
  }
}

package linea.domain

import org.hyperledger.besu.datatypes.TransactionType

fun TransactionType.toDomain(): linea.domain.TransactionType {
  return when (this) {
    TransactionType.FRONTIER -> linea.domain.TransactionType.FRONTIER
    TransactionType.EIP1559 -> linea.domain.TransactionType.EIP1559
    TransactionType.ACCESS_LIST -> linea.domain.TransactionType.ACCESS_LIST
    TransactionType.BLOB -> linea.domain.TransactionType.BLOB
    TransactionType.DELEGATE_CODE -> linea.domain.TransactionType.DELEGATE_CODE
  }
}

fun linea.domain.TransactionType.toBesu(): TransactionType {
  return when (this) {
    linea.domain.TransactionType.FRONTIER -> TransactionType.FRONTIER
    linea.domain.TransactionType.EIP1559 -> TransactionType.EIP1559
    linea.domain.TransactionType.ACCESS_LIST -> TransactionType.ACCESS_LIST
    linea.domain.TransactionType.BLOB -> TransactionType.BLOB
    linea.domain.TransactionType.DELEGATE_CODE -> TransactionType.DELEGATE_CODE
  }
}

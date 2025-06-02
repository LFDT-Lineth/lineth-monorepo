package net.consensys.linea.transactionexclusion.app.api

enum class ApiMethod(val method: String) {
  LINEA_SAVE_REJECTED_TRANSACTION_V1("linea_saveRejectedTransactionV1"),
  LINEA_GET_TRANSACTION_EXCLUSION_STATUS_V1("linea_getTransactionExclusionStatusV1"),
}

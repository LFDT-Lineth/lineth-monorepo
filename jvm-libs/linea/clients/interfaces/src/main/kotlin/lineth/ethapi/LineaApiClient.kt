package lineth.ethapi

import lineth.domain.TransactionForEthCall
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface LineaApiClient {
  fun lineaEstimateGas(transaction: TransactionForEthCall): SafeFuture<ULong>
}

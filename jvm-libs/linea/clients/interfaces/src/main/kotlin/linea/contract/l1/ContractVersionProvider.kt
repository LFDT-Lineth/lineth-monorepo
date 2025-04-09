package linea.contract.l1

import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface ContractVersionProvider<T> {
  fun getVersion(): SafeFuture<T>
}

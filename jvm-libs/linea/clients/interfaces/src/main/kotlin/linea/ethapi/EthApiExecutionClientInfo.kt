package linea.ethapi

import tech.pegasys.teku.infrastructure.async.SafeFuture

interface EthApiExecutionClientInfo {
  fun ethProtocolVersion(): SafeFuture<Int>
  fun ethCoinbase(): SafeFuture<ByteArray>
  fun ethMining(): SafeFuture<Boolean>
}

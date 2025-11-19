package linea.ethapi

import tech.pegasys.teku.infrastructure.async.SafeFuture

interface EthApiChainIdClient {
  fun ethChainId(): SafeFuture<ULong>
}

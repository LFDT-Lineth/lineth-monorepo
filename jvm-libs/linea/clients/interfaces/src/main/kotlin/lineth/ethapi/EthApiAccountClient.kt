package lineth.ethapi

import lineth.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

interface EthApiAccountClient {
  fun ethGetBalance(address: ByteArray, blockParameter: BlockParameter): SafeFuture<BigInteger>
  fun ethGetTransactionCount(address: ByteArray, blockParameter: BlockParameter): SafeFuture<ULong>
}

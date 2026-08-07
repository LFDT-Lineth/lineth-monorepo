package lineth.contract.l1

import lineth.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ContractVersionProvider<T> {
  fun getVersion(blockParameter: BlockParameter = BlockParameter.Tag.LATEST): SafeFuture<T>
}

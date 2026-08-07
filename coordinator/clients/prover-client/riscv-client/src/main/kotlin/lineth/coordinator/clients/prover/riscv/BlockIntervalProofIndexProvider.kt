package lineth.coordinator.clients.prover.riscv

import lineth.crypto.HashFunction
import lineth.domain.BlockInterval
import lineth.domain.BlockIntervalProofIndex
import lineth.domain.StartBlockTimestampProvider

internal class BlockIntervalProofIndexProvider<Request>(
  private val hashFunction: HashFunction,
) : (Request) -> BlockIntervalProofIndex
  where Request : BlockInterval, Request : StartBlockTimestampProvider {
  override fun invoke(request: Request): BlockIntervalProofIndex {
    val content = request.toString().toByteArray()
    return BlockIntervalProofIndex(
      startBlockNumber = request.startBlockNumber,
      endBlockNumber = request.endBlockNumber,
      hash = hashFunction.hash(content),
      startBlockTimestamp = request.startBlockTimestamp,
    )
  }
}

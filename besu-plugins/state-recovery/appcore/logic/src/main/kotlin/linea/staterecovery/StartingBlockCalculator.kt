package linea.staterecovery

import linea.domain.BlockInterval
import linea.kotlin.minusCoercingUnderflow

fun startBlockToFetchFromL1(
  headBlockNumber: ULong,
  recoveryStartBlockNumber: ULong?,
  lookbackWindow: ULong
): ULong {
  if (recoveryStartBlockNumber == null) {
    return headBlockNumber + 1UL
  }

  return headBlockNumber
    .minusCoercingUnderflow(lookbackWindow)
    .coerceAtLeast(recoveryStartBlockNumber)
}

data class FetchingIntervals(
  val elInterval: BlockInterval?,
  val l1Interval: BlockInterval?
)

fun lookbackFetchingIntervals(
  headBlockNumber: ULong,
  recoveryStartBlockNumber: ULong?,
  lookbackWindow: ULong
): FetchingIntervals {
  if (recoveryStartBlockNumber == null || recoveryStartBlockNumber > headBlockNumber) {
    return FetchingIntervals(
      l1Interval = null,
      elInterval = BlockInterval(headBlockNumber.minusCoercingUnderflow(lookbackWindow - 1UL), headBlockNumber)
    )
  }
  if (headBlockNumber - lookbackWindow > recoveryStartBlockNumber) {
    return FetchingIntervals(
      l1Interval = BlockInterval(headBlockNumber.minusCoercingUnderflow(lookbackWindow - 1UL), headBlockNumber),
      elInterval = null
    )
  }

  return FetchingIntervals(
    l1Interval = BlockInterval(recoveryStartBlockNumber, headBlockNumber),
    elInterval = BlockInterval(
      headBlockNumber.minusCoercingUnderflow(lookbackWindow - 1UL),
      recoveryStartBlockNumber - 1UL
    )
  )
}

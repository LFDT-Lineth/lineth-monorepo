package net.consensys.zkevm.domain

import linea.forcedtx.ForcedTransactionInclusionResult
import kotlin.time.Instant

object ForcedTransactionRecordFactory {
  fun createForcedTransactionRecord(
    ftxNumber: ULong = 1UL,
    inclusionResult: ForcedTransactionInclusionResult = ForcedTransactionInclusionResult.BadNonce,
    simulatedExecutionBlockNumber: ULong = 100UL,
    simulatedExecutionBlockTimestamp: Instant = Instant.fromEpochSeconds(0),
    ftxBlockNumberDeadline: ULong = 200UL,
    ftxRollingHash: ByteArray = byteArrayOf(0x00),
    ftxRlp: ByteArray = byteArrayOf(0x00),
    proofStatus: ForcedTransactionRecord.ProofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
  ): ForcedTransactionRecord {
    return ForcedTransactionRecord(
      ftxNumber = ftxNumber,
      inclusionResult = inclusionResult,
      simulatedExecutionBlockNumber = simulatedExecutionBlockNumber,
      simulatedExecutionBlockTimestamp = simulatedExecutionBlockTimestamp,
      ftxBlockNumberDeadline = ftxBlockNumberDeadline,
      ftxRollingHash = ftxRollingHash,
      ftxRlp = ftxRlp,
      proofStatus = proofStatus,
    )
  }
}

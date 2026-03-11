package net.consensys.zkevm.domain

import linea.forcedtx.ForcedTransactionInclusionResult
import linea.kotlin.encodeHex
import kotlin.time.Instant

/**
 *  linea_getForcedTransactionInclusionStatus
 * {
 *     "blockNumber": "0xeff35f", // On which transaction was tried
 *     "from": "0x6221a9c005f6e47eb398fd867784cacfdcfff4e7",
 *     // inclusion result / the type of invalidity for each forced transaction;
 *     // for the executed valid transaction it is set to [invalidity.BadNonce]
 *     // cases: Included, BadNonce, BadBalance, BadPrecompile, TooManyLogs, FilteredAddressesFrom, FilteredAddressesTo, Phylax)
 *     "inclusionResult": "BadNonce"
 *     "transactionHash": "0xTRANSACTION_HASH",
 *   }
 *
 *  // Invalidity proof request
 *  {
 *    "ftxRLP": "0x...",
 *    "ftxNumber": 12345, // decimal encoding
 *    "prevFtxRollingHash": "0x...",
 *    "ftxBlockNumberDeadline": 67890,   // decimal encoding
 *    "invalidityType": "BadNonce",
 *    // {BadNonce, BadBalance, BadPrecompile, TooManyLogs, FilteredAddresses}
 *    "parentBlockHash": "0x...",
 *    "zkParentStateRootHash": "0x...",
 *    "conflatedExecutionTracesFile": "path/to/traces.lt ( one when BadPrecompile, TooManyLogs)",
 *
 *    // "case of BadNonce, BadBalance,"
 *    "accountProof": "shomei reponse of rollup_getProof(account address)"
 *    // case of "BadPrecompile, TooManyLogs,"
 *    "zkStateMerkleProof": "shomei trace full as off today",// require shomei to trace a block that does not exist
 *    // case of FilteredAddresses, accountMerkleProof=null, zkStateMerkleProof=null
 *    "simulatedExecutionBlockNumber": 12123, // lastFinalizedBlockNumber + 1
 *    "simulatedExecutionBlockTimestamp": 123123
 * }
 *
 */
data class ForcedTransactionRecord(
  val ftxNumber: ULong,
  val inclusionResult: ForcedTransactionInclusionResult,
  val simulatedExecutionBlockNumber: ULong,
  val simulatedExecutionBlockTimestamp: Instant,
  val ftxBlockNumberDeadline: ULong,
  val ftxRollingHash: ByteArray,
  val ftxRlp: ByteArray,
  val proofStatus: ProofStatus,
) {
  enum class ProofStatus {
    /** Invalidity proof has not been requested yet, execution status just inserted into the DB */
    UNREQUESTED,

    /** Invalidity proof has been requested from the prover */
    REQUESTED,

    /** Invalidity proof has been successfully generated */
    PROVEN,
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForcedTransactionRecord

    if (ftxNumber != other.ftxNumber) return false
    if (inclusionResult != other.inclusionResult) return false
    if (simulatedExecutionBlockNumber != other.simulatedExecutionBlockNumber) return false
    if (simulatedExecutionBlockTimestamp != other.simulatedExecutionBlockTimestamp) return false
    if (ftxBlockNumberDeadline != other.ftxBlockNumberDeadline) return false
    if (!ftxRollingHash.contentEquals(other.ftxRollingHash)) return false
    if (!ftxRlp.contentEquals(other.ftxRlp)) return false
    if (proofStatus != other.proofStatus) return false

    return true
  }

  override fun hashCode(): Int {
    var result = ftxNumber.hashCode()
    result = 31 * result + inclusionResult.hashCode()
    result = 31 * result + simulatedExecutionBlockNumber.hashCode()
    result = 31 * result + simulatedExecutionBlockTimestamp.hashCode()
    result = 31 * result + ftxBlockNumberDeadline.hashCode()
    result = 31 * result + ftxRollingHash.contentHashCode()
    result = 31 * result + ftxRlp.contentHashCode()
    result = 31 * result + proofStatus.hashCode()
    return result
  }

  override fun toString(): String {
    return "ForcedTransactionRecord(" +
      "ftxNumber=$ftxNumber, " +
      "inclusionResult=$inclusionResult, " +
      "simulatedExecutionBlockNumber=$simulatedExecutionBlockNumber, " +
      "simulatedExecutionBlockTimestamp=$simulatedExecutionBlockTimestamp, " +
      "ftxBlockNumberDeadline=$ftxBlockNumberDeadline, " +
      "ftxRollingHash=${ftxRollingHash.encodeHex()}, " +
      "ftxRlp=${ftxRlp.encodeHex()}, " +
      "proofStatus=$proofStatus" +
      ")"
  }
}

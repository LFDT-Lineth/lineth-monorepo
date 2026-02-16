package net.consensys.zkevm.domain

import linea.forcedtx.ForcedTransactionInclusionResult
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
  val proofStatus: ProofStatus,
  val proofIndex: InvalidityProofIndex? = null,
) {
  enum class ProofStatus {
    /** Invalidity proof has not been requested yet, execution status just inserted into the DB */
    UNREQUESTED,

    /** Invalidity proof has been requested from the prover */
    REQUESTED,

    /** Invalidity proof has been successfully generated */
    PROVEN,
  }
}

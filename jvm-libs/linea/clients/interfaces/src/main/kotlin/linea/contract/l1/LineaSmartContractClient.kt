package linea.contract.l1

import linea.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Instant

enum class LineaRollupContractVersion : Comparable<LineaRollupContractVersion> {
  V6, // more efficient data submission and new events for state recovery
  V7, // Native Yield (no practical changes for the coordinator)
  V8, // Forced Transactions
}

enum class LineaValidiumContractVersion : Comparable<LineaValidiumContractVersion> {
  V1,
}

interface LineaSmartContractClientReadOnly {

  fun getAddress(): String

  /**
   * Get the current L2 block number
   */
  fun finalizedL2BlockNumber(blockParameter: BlockParameter = BlockParameter.Tag.LATEST): SafeFuture<ULong>

  fun getMessageRollingHash(
    blockParameter: BlockParameter = BlockParameter.Tag.LATEST,
    messageNumber: Long,
  ): SafeFuture<ByteArray>

  /**
   * Checks if a blob's shnarf is already present in the smart contract
   * It meant blob was sent to l1 and accepted by the smart contract.
   * Note: snarf in the future may be cleanned up after finalization.
   */
  fun isBlobShnarfPresent(
    blockParameter: BlockParameter = BlockParameter.Tag.LATEST,
    shnarf: ByteArray,
  ): SafeFuture<Boolean>

  /**
   * Gets Type 2 StateRootHash for Linea Block
   */
  fun blockStateRootHash(blockParameter: BlockParameter, lineaL2BlockNumber: ULong): SafeFuture<ByteArray>
}

interface LineaRollupSmartContractClientReadOnly :
  LineaSmartContractClientReadOnly,
  ContractVersionProvider<LineaRollupContractVersion>

data class LineaRollupFinalizedState(
  val blockNumber: ULong,
  val blockTimestamp: Instant,
  val messageNumber: ULong,
  val forcedTransactionNumber: ULong,
)

interface LineaRollupSmartContractClientReadOnlyFinalizedStateProvider {
  /**
   * Provides the latest finalized state.
   * It relies on Linea contract V8 FinalizedStateUpdated event
   *
   * @throws UnsupportedOperationException when contract is not yet upgraded to V8 or when 1st event was not emitted yet
   */
  fun getLatestFinalizedState(blockParameter: BlockParameter): SafeFuture<LineaRollupFinalizedState>
}

interface LineaValidiumSmartContractClientReadOnly :
  LineaSmartContractClientReadOnly,
  ContractVersionProvider<LineaValidiumContractVersion>

/**
 * Polls contract's version until contract's is equal or greater than target version.
 *
 * This is useful for components that depend on a Contract upcoming feature and
 * need to wait for contract upgrade before starting normal operation.
 */
interface ContractVersionAwaiter<VersionType : Comparable<VersionType>> {
  /**
   * Waits until contract version reaches target version
   *
   * @param minTargetVersion minimum contractVersion to wait for
   * @param highestBlockTag Highest block tag (LATEST, SAFE, FINALIZED) to consider for version check
   * @param timeout Maximum time to wait for contractVersion to reach minTargetVersion
   *
   * @return Future that completes with the target version once reached, with
   */
  fun awaitVersion(
    minTargetVersion: VersionType,
    highestBlockTag: BlockParameter,
    timeout: Duration? = null,
  ): SafeFuture<VersionType>
}

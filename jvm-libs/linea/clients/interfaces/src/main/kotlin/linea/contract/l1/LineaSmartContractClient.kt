package linea.contract.l1

import linea.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class LineaRollupContractVersion : Comparable<LineaRollupContractVersion> {
  V6, // more efficient data submission and new events for state recovery
  V7, // Native Yield (no practical changes for the coordinator)
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

interface LineaValidiumSmartContractClientReadOnly :
  LineaSmartContractClientReadOnly,
  ContractVersionProvider<LineaValidiumContractVersion>

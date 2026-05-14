/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api.beacon

import maru.api.BeaconStateNotFoundException
import maru.api.BlockNotFoundException
import maru.api.ChainDataProvider
import maru.api.HandlerException
import maru.core.BeaconState
import maru.core.SealedBeaconBlock
import maru.core.Validator
import maru.extensions.fromHexToByteArray
import maru.extensions.isPositiveNumber

fun getBlock(
  blockId: String,
  chainDataProvider: ChainDataProvider,
): SealedBeaconBlock {
  val maruSealedBeaconBlock =
    try {
      when {
        blockId.lowercase() == "head" -> chainDataProvider.getLatestBeaconBlock()
        blockId.lowercase() == "finalized" -> chainDataProvider.getLatestBeaconBlock()
        blockId.lowercase() == "genesis" -> chainDataProvider.getGenesisBeaconBlock()
        blockId.isPositiveNumber() -> chainDataProvider.getBeaconBlockByNumber(blockId.toLong().toULong())
        blockId.lowercase().startsWith("0x") -> chainDataProvider.getBeaconBlockByBlockRoot(blockId)
        else -> throw HandlerException(400, "Invalid block ID: $blockId")
      }
    } catch (e: BlockNotFoundException) {
      throw HandlerException(404, "Block not found")
    }
  return maruSealedBeaconBlock
}

fun getBeaconState(
  stateId: String,
  chainDataProvider: ChainDataProvider,
): BeaconState =
  try {
    when {
      stateId.lowercase() == "head" -> chainDataProvider.getLatestBeaconState()
      stateId.lowercase() == "finalized" -> throw HandlerException(
        400,
        "Finalized state is not supported for validators endpoint",
      )
      stateId.lowercase() == "genesis" -> chainDataProvider.getGenesisBeaconState()
      stateId.lowercase().startsWith("0x") -> chainDataProvider.getBeaconStateByStateRoot(stateId.fromHexToByteArray())
      else -> throw HandlerException(400, "Invalid state ID: $stateId")
    }
  } catch (e: BeaconStateNotFoundException) {
    throw HandlerException(404, "State not found")
  }

fun getValidator(
  validatorId: String,
  beaconState: BeaconState,
): IndexedValue<Validator> {
  val indexedValidator =
    when {
      validatorId.isPositiveNumber() -> beaconState.validators.withIndex().find { it.index == validatorId.toInt() }
      validatorId.lowercase().startsWith(prefix = "0x") -> {
        val address = validatorId.fromHexToByteArray()
        beaconState.validators.withIndex().find { it.value.address.contentEquals(address) }
      }
      else -> throw HandlerException(400, "Invalid validator ID: $validatorId")
    }
  if (indexedValidator == null) {
    throw HandlerException(404, "Validator not found")
  }
  return indexedValidator
}

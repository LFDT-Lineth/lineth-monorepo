/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.validation.BeaconBlockValidatorFactory
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockImporter
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockValidator
import org.hyperledger.besu.consensus.qbft.core.types.QbftProtocolSchedule

class QbftProtocolScheduleAdapter(
  private val blockImporter: QbftBlockImporter,
  private val beaconBlockValidatorFactory: BeaconBlockValidatorFactory,
) : QbftProtocolSchedule {
  override fun getBlockImporter(blockHeader: QbftBlockHeader): QbftBlockImporter = blockImporter

  override fun getBlockValidator(blockHeader: QbftBlockHeader): QbftBlockValidator {
    val beaconBlockHeader = blockHeader.toBeaconBlockHeader()
    return QbftBlockValidatorAdapter(beaconBlockValidatorFactory.createValidatorForBlock(beaconBlockHeader))
  }
}

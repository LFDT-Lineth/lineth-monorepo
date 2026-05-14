/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.ValidatorProvider
import maru.consensus.qbft.toAddress
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader
import org.hyperledger.besu.consensus.qbft.core.types.QbftValidatorProvider
import org.hyperledger.besu.datatypes.Address

/**
 * Adapter to convert a [ValidatorProvider] to a [QbftValidatorProvider].
 */
class QbftValidatorProviderAdapter(
  private val validatorProvider: ValidatorProvider,
) : QbftValidatorProvider {
  override fun getValidatorsAfterBlock(header: QbftBlockHeader): Collection<Address> =
    validatorProvider.getValidatorsAfterBlock(header.toBeaconBlockHeader().number).get().map { it.toAddress() }

  override fun getValidatorsForBlock(header: QbftBlockHeader): Collection<Address> =
    validatorProvider.getValidatorsForBlock(header.toBeaconBlockHeader().number).get().map { it.toAddress() }
}

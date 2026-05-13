/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader
import org.hyperledger.besu.consensus.qbft.core.types.QbftValidatorModeTransitionLogger

class QbftValidatorModeTransitionLoggerAdapter : QbftValidatorModeTransitionLogger {
  override fun logTransitionChange(parentHeader: QbftBlockHeader) {
  }
}

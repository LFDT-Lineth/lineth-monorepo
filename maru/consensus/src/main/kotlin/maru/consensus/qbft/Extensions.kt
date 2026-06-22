/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.core.BeaconBlockHeader
import maru.core.Validator
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.datatypes.Address

fun BeaconBlockHeader.toConsensusRoundIdentifier(): ConsensusRoundIdentifier =
  ConsensusRoundIdentifier(this.number.toLong(), this.round.toInt())

fun Validator.toAddress(): Address = Address.wrap(Bytes.wrap(address))

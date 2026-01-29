/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.forced;

import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;

/**
 * Represents the inclusion status of a forced transaction.
 *
 * @param forcedTransactionNumber Unique identifier for the forced transaction
 * @param transactionHash The transaction hash
 * @param from The sender address
 * @param blockNumber The block number where the transaction was tried (final outcome)
 * @param blockTimestamp The timestamp of the block (seconds since epoch)
 * @param inclusionResult The result of the inclusion attempt
 */
public record ForcedTransactionStatus(
    long forcedTransactionNumber,
    Hash transactionHash,
    Address from,
    long blockNumber,
    long blockTimestamp,
    ForcedTransactionInclusionResult inclusionResult) {}

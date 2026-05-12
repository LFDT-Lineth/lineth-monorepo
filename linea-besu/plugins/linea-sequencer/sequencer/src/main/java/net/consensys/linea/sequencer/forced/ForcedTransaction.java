/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.forced;

import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;

/**
 * Represents a forced transaction submitted via linea_sendForcedRawTransaction.
 *
 * @param forcedTransactionNumber Unique identifier for the forced transaction
 * @param txHash The transaction hash
 * @param transaction The decoded transaction
 * @param deadlineBlockNumber Block number deadlineBlockNumber for inclusion (TODO: used for Phylax
 *     selector)
 */
public record ForcedTransaction(
    long forcedTransactionNumber, Hash txHash, Transaction transaction, long deadlineBlockNumber) {}

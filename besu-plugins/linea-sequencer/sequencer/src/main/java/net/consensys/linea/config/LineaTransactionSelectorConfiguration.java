/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.config;

import java.util.Map;
import java.util.Set;
import lombok.Builder;
import net.consensys.linea.plugins.LineaOptionsConfiguration;
import net.consensys.linea.sequencer.txselection.selectors.TransactionEventFilter;
import org.hyperledger.besu.datatypes.Address;

/** The Linea transaction selectors configuration. */
@Builder(toBuilder = true)
public record LineaTransactionSelectorConfiguration(
    int maxBlockCallDataSize,
    int overLinesLimitCacheSize,
    long maxGasPerBlock,
    long maxBundleGasPerBlock,
    long maxBundlePoolSizeBytes,
    String eventsDenyListPath,
    Map<Address, Set<TransactionEventFilter>> eventsDenyList,
    String eventsBundleDenyListPath,
    Map<Address, Set<TransactionEventFilter>> eventsBundleDenyList)
    implements LineaOptionsConfiguration {}

/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.config;

import java.util.Set;
import lombok.Builder;
import net.consensys.linea.plugins.LineaOptionsConfiguration;
import org.hyperledger.besu.datatypes.Address;

/**
 * The Linea transaction pool validation configuration.
 *
 * @param denyListPath the path to the file containing the addresses that are denied.
 * @param deniedAddresses the set of addresses that are denied.
 * @param bundleOverridingDenyListPath the path to the file containing the addresses that are denied
 *     for bundles.
 * @param bundleDeniedAddresses the set of addresses that are denied for bundles.
 * @param maxTxGasLimit the maximum gas limit allowed for transactions
 * @param maxTxCalldataSize the maximum size of calldata allowed for transactions
 * @param txPoolSimulationCheckApiEnabled flag to enable/disable simulation for tx received via RPC.
 * @param txPoolSimulationCheckP2pEnabled flag to enable/disable simulation for tx received via P2P.
 */
@Builder(toBuilder = true)
public record LineaTransactionPoolValidatorConfiguration(
    String denyListPath,
    Set<Address> deniedAddresses,
    String bundleOverridingDenyListPath,
    Set<Address> bundleDeniedAddresses,
    int maxTxGasLimit,
    Integer maxTxCalldataSize,
    boolean txPoolSimulationCheckApiEnabled,
    boolean txPoolSimulationCheckP2pEnabled)
    implements LineaOptionsConfiguration {}

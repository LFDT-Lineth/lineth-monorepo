/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package net.consensys.linea.zktracer;

import static net.consensys.linea.zktracer.Fork.*;
import static net.consensys.linea.zktracer.Trace.ETHEREUM_GAS_LIMIT_MAXIMUM;
import static net.consensys.linea.zktracer.Trace.ETHEREUM_GAS_LIMIT_MINIMUM;
import static net.consensys.linea.zktracer.Trace.LINEA_CHAIN_ID;
import static net.consensys.linea.zktracer.Trace.LINEA_GAS_LIMIT_MAXIMUM;
import static net.consensys.linea.zktracer.Trace.LINEA_GAS_LIMIT_MINIMUM;
import static net.consensys.linea.zktracer.Trace.LINEA_SEPOLIA_CHAIN_ID;

import java.math.BigInteger;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;

/**
 * Provides s single point of configuration for running the tracer on different chains which, aside
 * from Linea mainnet, are all currently for the purposes of testing.
 */
public class ChainConfig {
  /**
   * Represents Linea mainnet as it stands today which enforces the block gas limit (currently two
   * billion). As the name suggest, this is only intended for testing purposes.
   */
  public static final ChainConfig MAINNET_OSAKA_TESTCONFIG = MAINNET_TESTCONFIG(OSAKA);

  public static final int DEVNET_CHAIN_ID = 59139;

  public static final ChainConfig MAINNET_TESTCONFIG(final Fork fork) {
    return MAINNET_TESTCONFIG(fork, true);
  }

  public static final ChainConfig MAINNET_TESTCONFIG(
      final Fork fork, final boolean gasLimitEnabled) {
    return TESTCONFIG(fork, LINEA_CHAIN_ID, gasLimitEnabled);
  }

  public static final ChainConfig SEPOLIA_TESTCONFIG(final Fork fork) {
    return SEPOLIA_TESTCONFIG(fork, true);
  }

  public static final ChainConfig SEPOLIA_TESTCONFIG(final Fork fork, boolean gasLimitEnabled) {
    return TESTCONFIG(fork, LINEA_SEPOLIA_CHAIN_ID, gasLimitEnabled);
  }

  public static final ChainConfig TESTCONFIG(
      final Fork fork, int chainId, final boolean gasLimitEnabled) {
    return new ChainConfig(
        fork,
        chainId,
        gasLimitEnabled,
        BigInteger.valueOf(LINEA_GAS_LIMIT_MINIMUM),
        BigInteger.valueOf(LINEA_GAS_LIMIT_MAXIMUM),
        LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT);
  }

  public static ChainConfig DEVNET_TESTCONFIG(final Fork fork) {
    return new ChainConfig(
        fork,
        DEVNET_CHAIN_ID,
        true,
        BigInteger.valueOf(LINEA_GAS_LIMIT_MINIMUM),
        BigInteger.valueOf(LINEA_GAS_LIMIT_MAXIMUM),
        LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT);
  }

  public final Fork fork;

  /** ChainID for this chain */
  public final BigInteger id;

  /** Determines whether block gas limit is enabled (or not). */
  public final boolean fixedGasLimitEnabled;

  /** Determines minimum gas limit for this chain */
  public final BigInteger gasLimitMinimum;

  /** Determines maximum gas limit for this chain */
  public final BigInteger gasLimitMaximum;

  /** Determines the bridge configuration being used. */
  public final LineaL1L2BridgeSharedConfiguration bridgeConfiguration;

  private ChainConfig(
      Fork fork,
      int chainId,
      boolean gasLimitEnabled,
      BigInteger gasLimitMinimum,
      BigInteger gasLimitMaximum,
      LineaL1L2BridgeSharedConfiguration bridgeConfig) {
    this(
        fork,
        BigInteger.valueOf(chainId),
        gasLimitEnabled,
        gasLimitMinimum,
        gasLimitMaximum,
        bridgeConfig);
  }

  private ChainConfig(
      Fork fork,
      BigInteger chainId,
      boolean fixedGasLimitEnabled,
      BigInteger gasLimitMinimum,
      BigInteger gasLimitMaximum,
      LineaL1L2BridgeSharedConfiguration bridgeConfig) {
    // Sanity check chainId is non-negative.
    if (chainId.compareTo(BigInteger.ZERO) < 0) {
      throw new IllegalArgumentException("invalid chain id (" + chainId + ")");
    }
    //
    this.fork = fork;
    this.id = chainId;
    this.fixedGasLimitEnabled = fixedGasLimitEnabled;
    this.gasLimitMinimum = gasLimitMinimum;
    this.gasLimitMaximum = gasLimitMaximum;
    this.bridgeConfiguration = bridgeConfig;
  }

  public static ChainConfig FORK_LINEA_CHAIN(
      Fork fork, LineaL1L2BridgeSharedConfiguration bridgeConfig, BigInteger chainId) {
    return new ChainConfig(
        fork,
        chainId,
        true,
        BigInteger.valueOf(LINEA_GAS_LIMIT_MINIMUM),
        BigInteger.valueOf(LINEA_GAS_LIMIT_MAXIMUM),
        bridgeConfig);
  }

  /**
   * Construct a suitable configuration representing a specific fork of Ethereum mainnet.
   *
   * @param fork
   * @return
   */
  public static ChainConfig ETHEREUM_CHAIN(Fork fork) {
    return new ChainConfig(
        fork,
        1,
        false,
        BigInteger.valueOf(ETHEREUM_GAS_LIMIT_MINIMUM),
        ETHEREUM_GAS_LIMIT_MAXIMUM,
        LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT);
  }
}

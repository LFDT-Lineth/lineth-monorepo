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

package net.consensys.linea.plugins.config;

import com.google.common.base.MoreObjects;
import net.consensys.linea.plugins.LineaCliOptions;
import net.consensys.linea.plugins.config.converters.AddressConverter;
import net.consensys.linea.plugins.config.converters.BytesConverter;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import picocli.CommandLine;

/** The Linea L1 L2 Bridge CLI options. */
public class LineaL1L2BridgeSharedCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "l1-l2-bridge-shared-config";

  private static final String L1L2_BRIDGE_CONTRACT = "--plugin-linea-l1l2-bridge-contract";
  private static final String L1L2_BRIDGE_TOPIC = "--plugin-linea-l1l2-bridge-topic";

  @CommandLine.Option(
      names = {L1L2_BRIDGE_CONTRACT},
      paramLabel = "<ADDRESS>",
      converter = AddressConverter.class,
      description = "The address of the L1 L2 bridge contract (default: ${DEFAULT-VALUE})")
  private Address l1l2BridgeContract = Address.ZERO;

  @CommandLine.Option(
      names = {L1L2_BRIDGE_TOPIC},
      paramLabel = "<HEX_STRING>",
      converter = BytesConverter.class,
      description = "The log topic of the L1 L2 bridge (default: ${DEFAULT-VALUE})")
  private Bytes32 l1l2BridgeTopic = Bytes32.ZERO;

  private LineaL1L2BridgeSharedCliOptions() {}

  /**
   * Create Linea cli options.
   *
   * @return the Linea cli options
   */
  public static LineaL1L2BridgeSharedCliOptions create() {
    return new LineaL1L2BridgeSharedCliOptions();
  }

  /**
   * Linea cli options from config.
   *
   * @param config the config
   * @return the Linea cli options
   */
  public static LineaL1L2BridgeSharedCliOptions fromConfig(
      final LineaL1L2BridgeSharedConfiguration config) {
    final LineaL1L2BridgeSharedCliOptions options = create();
    options.l1l2BridgeContract = config.contract();
    options.l1l2BridgeTopic = config.topic();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public LineaL1L2BridgeSharedConfiguration toDomainObject() {
    return LineaL1L2BridgeSharedConfiguration.builder()
        .contract(l1l2BridgeContract)
        .topic(l1l2BridgeTopic)
        .build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(L1L2_BRIDGE_CONTRACT, l1l2BridgeContract.toHexString())
        .add(L1L2_BRIDGE_TOPIC, l1l2BridgeTopic.toHexString())
        .toString();
  }
}

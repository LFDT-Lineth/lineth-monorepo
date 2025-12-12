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

import lombok.Builder;
import net.consensys.linea.plugins.LineaOptionsConfiguration;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;

/** The Linea L1 L2 bridge configuration. */
@Builder(toBuilder = true)
public record LineaL1L2BridgeSharedConfiguration(Address contract, Bytes32 topic)
    implements LineaOptionsConfiguration {

  // = Hash(MessageSent(address,address,uint256,uint256,uint256,bytes,bytes32))
  private static Bytes32 LINEA_L2L1TOPIC =
      Bytes32.fromHexString("0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c");

  private static final Address SEPOLIA_L2L1LOGS_SMC =
      Address.fromHexString("0x971e727e956690b9957be6d51Ec16E73AcAC83A7");

  public static final LineaL1L2BridgeSharedConfiguration TEST_DEFAULT =
      LineaL1L2BridgeSharedConfiguration.builder()
          .contract(Address.fromHexString("0x7e577e577e577e577e577e577e577e577e577e57"))
          .topic(LINEA_L2L1TOPIC)
          .build();

  public static final LineaL1L2BridgeSharedConfiguration EMPTY =
      LineaL1L2BridgeSharedConfiguration.builder()
          .contract(Address.ZERO)
          .topic(Bytes32.ZERO)
          .build();
}

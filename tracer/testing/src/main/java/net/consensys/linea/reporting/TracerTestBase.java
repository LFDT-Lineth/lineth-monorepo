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
package net.consensys.linea.reporting;

import static net.consensys.linea.zktracer.Fork.*;

import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.opcode.OpCodes;
import org.junit.jupiter.api.BeforeAll;

public class TracerTestBase {
  public static ChainConfig chainConfig;
  public static Fork fork;
  public static OpCodes opcodes;

  @BeforeAll
  public static void init() {
    // Configure chain information and fork before any tests are run, including any methods used as
    // MethodSource.
    TracerTestBase.chainConfig = ChainConfig.MAINNET_TESTCONFIG(getForkOrDefault(OSAKA));
    TracerTestBase.fork = TracerTestBase.chainConfig.fork;
    TracerTestBase.opcodes = OpCodes.load(fork);
  }

  public static Fork getForkOrDefault(Fork defaultFork) {
    final String fork = System.getenv("ZKEVM_FORK");
    if (fork != null) {
      return fromString(fork);
    }
    return defaultFork;
  }
}

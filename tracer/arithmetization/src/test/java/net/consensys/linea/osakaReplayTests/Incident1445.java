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

package net.consensys.linea.osakaReplayTests;

import static net.consensys.linea.ReplayTestTools.replay;
import static net.consensys.linea.zktracer.ChainConfig.MAINNET_TESTCONFIG;
import static net.consensys.linea.zktracer.ChainConfig.SEPOLIA_TESTCONFIG;
import static net.consensys.linea.zktracer.Fork.OSAKA;

import net.consensys.linea.reporting.TracerTestBase;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.parallel.Execution;
import org.junit.jupiter.api.parallel.ExecutionMode;

@Execution(ExecutionMode.SAME_THREAD)
@Tag("replay")
public class Incident1445 extends TracerTestBase {

  // Incident 1445 was an issue where Besu was not executing the system transactions in the tracer
  // service.
  // It triggered a divergence in execution paths between the tracer node and the Besu Shomei node.
  // And therefore discrepancies in state updates between the tracer's trace and the Shomei's one.
  @Test
  void block_28279135_28279249(TestInfo testInfo) {
    replay(
        MAINNET_TESTCONFIG(OSAKA),
        "osaka/incident-1445-28279135-28279249.json.gz",
        testInfo,
        false);
  }

  // Faulty block from block_28279135_28279249 test
  @Test
  void block_28279180(TestInfo testInfo) {
    replay(
        MAINNET_TESTCONFIG(OSAKA), "osaka/incident-1445-28279180.mainnet.json.gz", testInfo, false);
  }

  // Faulty block from block_28279135_28279249 test ran with Besu node to trigger the same execution
  // as in prod

  // TODO: reenable when Besu 25.12.0-linea4 contains the fix for incident 1445
  // https://github.com/Consensys/linea-monorepo/issues/2194
  /*
    @Test
    void block_28279180_runWithBesu(TestInfo testInfo) {
      replay(MAINNET_TESTCONFIG(OSAKA), "osaka/incident-1445-28279180.mainnet.json.gz", testInfo, false, true);
    }
  */

  // Issue on Sepolia
  @Test
  void block_23985771(TestInfo testInfo) {
    replay(
        SEPOLIA_TESTCONFIG(OSAKA), "osaka/incident-1445-23985771.sepolia.json.gz", testInfo, false);
  }

  // TODO: reenable when Besu 25.12.0-linea4 contains the fix for incident 1445
  // https://github.com/Consensys/linea-monorepo/issues/2194
  /*  @Test
  void block_23985771_runWithBesu(TestInfo testInfo) {
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/incident-1445-23985771.sepolia.json.gz", testInfo, false, true);
  }*/
}

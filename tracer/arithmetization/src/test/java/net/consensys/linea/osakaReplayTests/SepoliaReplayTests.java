/*
 * Copyright ConsenSys Inc.
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
import static net.consensys.linea.zktracer.ChainConfig.SEPOLIA_TESTCONFIG;
import static net.consensys.linea.zktracer.Fork.OSAKA;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@Tag("replay")
@ExtendWith(UnitTestWatcher.class)
public class SepoliaReplayTests extends TracerTestBase {

  @Test
  void block_21511543(TestInfo testInfo) {
    // 44 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511543.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511544(TestInfo testInfo) {
    // 7 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511544.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511580(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511580.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511612(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511612.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511792(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511792.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511810(TestInfo testInfo) {
    // 37 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511810.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511877(TestInfo testInfo) {
    // 9 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511877.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511882(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511882.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511923(TestInfo testInfo) {
    // 4 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511923.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511928(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511928.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511973(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511973.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511976(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511976.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21511985(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21511985.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21512014(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21512014.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21512018(TestInfo testInfo) {
    // 2 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21512018.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21512024(TestInfo testInfo) {
    // 5 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21512024.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21514640(TestInfo testInfo) {
    // 44 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21514640.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21515399(TestInfo testInfo) {
    // 7 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21515399.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21515617(TestInfo testInfo) {
    // 9 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21515617.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21515753(TestInfo testInfo) {
    // 7 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21515753.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21516095(TestInfo testInfo) {
    // 4 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21516095.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21516210(TestInfo testInfo) {
    // 44 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21516210.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21516247(TestInfo testInfo) {
    // 9 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21516247.sepolia.json.gz", testInfo);
  }

  @Test
  void block_21516576(TestInfo testInfo) {
    // 8 internal calls
    replay(SEPOLIA_TESTCONFIG(OSAKA), "osaka/21516576.sepolia.json.gz", testInfo);
  }
}

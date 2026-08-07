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
package lineth.pragueRunAsOsakaReplayTests;

import static lineth.ReplayTestTools.replay;
import static lineth.zktracer.ChainConfig.SEPOLIA_TESTCONFIG;
import static lineth.zktracer.Fork.OSAKA;

import lineth.UnitTestWatcher;
import lineth.reporting.TracerTestBase;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@Tag("replay")
@ExtendWith(UnitTestWatcher.class)
public class BlockHashTests extends TracerTestBase {

  @Test
  void someHistoricalHashesAreChecked(TestInfo testInfo) {
    replay(SEPOLIA_TESTCONFIG(OSAKA), "pragueRunAsOsaka/19562398.sepolia.prague.json.gz", testInfo);
  }

  /**
   * The purpose of this second replay tests is to provide two consecutive conflation to the prover,
   * with BLOCKHASH checking the "historical hashes" in both conflations.
   */
  @Disabled
  @Test
  void conflationFollowingThePreviousOneWithAgainHistoricalBlockhashesChecked(TestInfo testInfo) {
    replay(
        SEPOLIA_TESTCONFIG(OSAKA),
        "pragueRunAsOsaka/19562399-19562417.sepolia.prague.json.gz",
        testInfo);
  }
}

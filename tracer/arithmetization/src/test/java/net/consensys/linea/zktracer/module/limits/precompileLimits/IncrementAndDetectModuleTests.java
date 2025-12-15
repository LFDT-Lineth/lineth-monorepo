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

package net.consensys.linea.zktracer.module.limits.precompileLimits;

import static java.lang.Integer.MAX_VALUE;
import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.zktracer.ZkTracer;
import net.consensys.linea.zktracer.container.module.IncrementAndDetectModule;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class IncrementAndDetectModuleTests extends TracerTestBase {
  @Test
  void legalThenTwoIllegals(TestInfo testInfo) {
    final ZkTracer state = new ZkTracer(chainConfig);
    final IncrementAndDetectModule countingOnlyModule = state.getHub().modexpEffectiveCall();

    countingOnlyModule.updateTally(1);

    countingOnlyModule.detectEvent();
    assertThat(countingOnlyModule.lineCount()).isEqualTo(MAX_VALUE);

    countingOnlyModule.detectEvent();
    assertThat(countingOnlyModule.lineCount()).isEqualTo(MAX_VALUE);

    countingOnlyModule.popTransactionBundle();
    assertThat(countingOnlyModule.lineCount()).isEqualTo(0);
  }

  @Test
  void legalIllegalLegal(TestInfo testInfo) {
    final ZkTracer state = new ZkTracer(chainConfig);
    final IncrementAndDetectModule countingOnlyModule = state.getHub().modexpEffectiveCall();

    countingOnlyModule.updateTally(1);

    countingOnlyModule.detectEvent();
    assertThat(countingOnlyModule.lineCount()).isEqualTo(MAX_VALUE);

    countingOnlyModule.updateTally(1);
    assertThat(countingOnlyModule.lineCount()).isEqualTo(MAX_VALUE);

    countingOnlyModule.popTransactionBundle();
    assertThat(countingOnlyModule.lineCount()).isEqualTo(0);
  }

  @Test
  void TwoIllegals(TestInfo testInfo) {
    final ZkTracer state = new ZkTracer(chainConfig);
    final IncrementAndDetectModule countingOnlyModule = state.getHub().modexpEffectiveCall();

    countingOnlyModule.detectEvent();
    assertThat(countingOnlyModule.lineCount()).isEqualTo(MAX_VALUE);

    countingOnlyModule.detectEvent();
    assertThat(countingOnlyModule.lineCount()).isEqualTo(MAX_VALUE);

    countingOnlyModule.popTransactionBundle();
    assertThat(countingOnlyModule.lineCount()).isEqualTo(0);
  }
}

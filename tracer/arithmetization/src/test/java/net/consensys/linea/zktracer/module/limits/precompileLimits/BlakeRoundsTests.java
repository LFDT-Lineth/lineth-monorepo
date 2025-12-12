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
import static net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpDataOperation.BLAKE2f_R_SIZE;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;
import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.zktracer.ZkTracer;
import net.consensys.linea.zktracer.module.limits.precompiles.BlakeRounds;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;

public class BlakeRoundsTests extends TracerTestBase {

  private static final Bytes ONE = leftPadTo(Bytes.minimalBytes(1), BLAKE2f_R_SIZE);
  private static final Bytes ZERO = leftPadTo(Bytes.minimalBytes(0), BLAKE2f_R_SIZE);
  private static final Bytes MAX_INTEGER = leftPadTo(Bytes.minimalBytes(MAX_VALUE), BLAKE2f_R_SIZE);
  private static final Bytes MAX_INTEGER_MO =
      leftPadTo(Bytes.minimalBytes(MAX_VALUE - 1), BLAKE2f_R_SIZE);
  private static final Bytes MAX_INTEGER_PO =
      leftPadTo(Bytes.minimalBytes((long) MAX_VALUE + 1), BLAKE2f_R_SIZE);

  @Test
  void checkWoCommit() {
    final ZkTracer state = new ZkTracer(chainConfig);
    final BlakeRounds blakeRounds = state.getHub().blakeModexpData().blakeRounds();

    blakeRounds.addPrecompileLimit(ONE);
    assertThat(blakeRounds.lineCount()).isEqualTo(1);

    blakeRounds.addPrecompileLimit(ZERO);
    assertThat(blakeRounds.lineCount()).isEqualTo(1);

    blakeRounds.addPrecompileLimit(MAX_INTEGER);
    assertThat(blakeRounds.lineCount()).isEqualTo(MAX_VALUE);

    blakeRounds.addPrecompileLimit(MAX_INTEGER_MO);
    assertThat(blakeRounds.lineCount()).isEqualTo(MAX_VALUE);

    blakeRounds.popTransactionBundle();
    assertThat(blakeRounds.lineCount()).isEqualTo(0);

    blakeRounds.addPrecompileLimit(MAX_INTEGER_MO);
    assertThat(blakeRounds.lineCount()).isEqualTo(MAX_INTEGER_MO.toInt());

    blakeRounds.popTransactionBundle();
    assertThat(blakeRounds.lineCount()).isEqualTo(0);

    blakeRounds.addPrecompileLimit(MAX_INTEGER_PO);
    assertThat(blakeRounds.lineCount()).isEqualTo(MAX_VALUE);
  }

  @Test
  void checkWithCommit() {
    final ZkTracer state = new ZkTracer(chainConfig);
    final BlakeRounds blakeRounds = state.getHub().blakeModexpData().blakeRounds();

    blakeRounds.addPrecompileLimit(ONE);
    assertThat(blakeRounds.lineCount()).isEqualTo(1);

    blakeRounds.commitTransactionBundle();
    assertThat(blakeRounds.lineCount()).isEqualTo(1);

    blakeRounds.addPrecompileLimit(MAX_INTEGER);
    assertThat(blakeRounds.lineCount()).isEqualTo(MAX_VALUE);
    blakeRounds.popTransactionBundle();
    assertThat(blakeRounds.lineCount()).isEqualTo(1);

    blakeRounds.addPrecompileLimit(MAX_INTEGER_MO);
    assertThat(blakeRounds.lineCount()).isEqualTo(MAX_VALUE);
    blakeRounds.popTransactionBundle();
    assertThat(blakeRounds.lineCount()).isEqualTo(1);

    blakeRounds.addPrecompileLimit(MAX_INTEGER_PO);
    assertThat(blakeRounds.lineCount()).isEqualTo(MAX_VALUE);
    blakeRounds.popTransactionBundle();
    assertThat(blakeRounds.lineCount()).isEqualTo(1);

    blakeRounds.addPrecompileLimit(ONE);
    assertThat(blakeRounds.lineCount()).isEqualTo(2);

    blakeRounds.commitTransactionBundle();
    assertThat(blakeRounds.lineCount()).isEqualTo(2);
  }
}

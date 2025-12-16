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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecmul;

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.CodeExecutionMethods.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.GasParameter.COST;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecmul.MemoryContents.WELL_FORMED_POINT_AND_NONTRIVIAL_MULTIPLIER;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.stream.Stream;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ReturnAtParameter;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallTests;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.provider.Arguments;

@Tag("prc-calltests")
public class Tests extends PrecompileCallTests<CallParameters> {
  // Set sample size with potential for override.
  private static final int ECMUL_SAMPLE_SIZE =
      Integer.parseInt(System.getenv().getOrDefault("PRC_CALLTESTS_SAMPLE_SIZE", "700"));

  public static Stream<Arguments> parameterGeneration() {
    return randomSampleByDayOfMonth(ECMUL_SAMPLE_SIZE, ParameterGeneration.parameterGeneration())
        .stream();
  }

  /** Non-parametric test to make sure things are working as expected. */
  @Test
  public void singleMessageCallTransactionTest(TestInfo testInfo) {
    CallParameters params =
        new CallParameters(
            CALL,
            COST,
            WELL_FORMED_POINT_AND_NONTRIVIAL_MULTIPLIER,
            CallDataSizeParameter.FULL,
            ReturnAtParameter.FULL,
            true);

    BytecodeCompiler rootCode =
        params.customPrecompileCallsSeparatedByReturnDataWipingOperation(chainConfig);
    if (params.willRevert()) revertWith(rootCode, 3 * WORD_SIZE, 2 * WORD_SIZE);

    runMessageCallTransactionWithProvidedCodeAsRootCode(rootCode, chainConfig, testInfo);
  }
}

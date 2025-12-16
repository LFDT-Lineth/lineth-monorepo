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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.hash;

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.CodeExecutionMethods.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.GasParameter.COST;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.RelativeRangePosition.*;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.stream.Stream;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.*;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallTests;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.provider.Arguments;

/**
 * <b>Happy path</b> tests for <b>SHA2-256</b>, <b>RIPEMD-160</b> and <b>IDENTITY</b>. The present
 * tests pertain to precompile calls where
 *
 * <p>- the precompile is provided with sufficient gas (ensuring <b>scenario/PRC_SUCCESS</b>)
 *
 * <p>- nothing <b>REVERT</b>s (thus value only matters in terms of pricing)
 *
 * <p>To avoid trivialities we pre-populate memory with nonzero values. We force interactions
 * between the <b>call data</b> range, the <b>return at</b> range and <b>return data</b> ranges in
 * the <b>OVERLAP</b> case.
 *
 * <p>After the call we interact with return data via <b>RETURNDATA[SIZE/COPY]</b> and <b>MLOAD</b>.
 *
 * <p>The tests do more: we then wipe the return data and start all over again with the next
 * precompile.
 *
 * <p>To give full details, we will test the following scenario which we call <b>happy path
 * precompile</b>:
 *
 * <p>- happy path precompile CALL
 *
 * <p>- play with (precompile) return data
 *
 * <p>- wipe return data
 *
 * <p>- (different) happy path precompile CALL
 *
 * <p>- play with (precompile) return data
 */
@Tag("prc-calltests")
public class Tests extends PrecompileCallTests<CallParameters> {
  // Set sample size with potential for override.
  private static final int HASH_SAMPLE_SIZE =
      Integer.parseInt(System.getenv().getOrDefault("PRC_CALLTESTS_SAMPLE_SIZE", "2000"));

  public static Stream<Arguments> parameterGeneration() {
    return randomSampleByDayOfMonth(HASH_SAMPLE_SIZE, ParameterGeneration.parameterGeneration())
        .stream();
  }

  /** Non-parametric test to make sure things are working as expected. */
  @Test
  public void singleMessageCallTransactionTest(TestInfo testInfo) {
    CallParameters params =
        new CallParameters(
            CALL,
            COST,
            HashPrecompile.IDENTITY,
            ValueParameter.ZERO,
            CallOffset.ALIGNED,
            CallSize.WORD,
            CallOffset.MISALIGNED,
            CallSize.WORD,
            new MemoryContents(),
            OVERLAP,
            true);

    BytecodeCompiler rootCode =
        params.customPrecompileCallsSeparatedByReturnDataWipingOperation(chainConfig);
    if (params.willRevert()) revertWith(rootCode, 3 * WORD_SIZE, 2 * WORD_SIZE);

    runMessageCallTransactionWithProvidedCodeAsRootCode(rootCode, chainConfig, testInfo);
  }
}

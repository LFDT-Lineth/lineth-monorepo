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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing;

import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.randomSampleByCurrentCommitHash;

import java.util.stream.Stream;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallTests;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.params.provider.Arguments;

@Tag("prc-calltests")
public class Tests extends PrecompileCallTests<CallParameters> {

  public static Stream<Arguments> parameterGeneration() {
    return randomSampleByCurrentCommitHash(ParameterGeneration.parameterGeneration()).stream();
  }

  // @Test
  // public void singleMessageCallTransactionTest() {
  //   CallParameters params =
  //       new CallParameters(
  //           CALL,
  //           COST,
  //           new MemoryContents(SmallPoint.INFINITY, LargePoint.INFINITY),
  //           new CallDataRange(0, TOTAL_NUMBER_OF_PAIRS_OF_POINTS - 1),
  //           ReturnAtParameter.FULL,
  //           true);
  //   BytecodeCompiler rootCode =
  // params.customPrecompileCallsSeparatedByReturnDataWipingOperation();
  //   if (params.willRevert()) revertWith(rootCode, 3 * WORD_SIZE, 2 * WORD_SIZE);
  //   runMessageCallTransactionWithProvidedCodeAsRootCode(rootCode);
  // }
}

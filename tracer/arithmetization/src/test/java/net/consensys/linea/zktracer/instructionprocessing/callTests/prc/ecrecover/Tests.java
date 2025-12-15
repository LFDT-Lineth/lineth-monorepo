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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecrecover;

import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.randomSampleByDayOfMonth;

import java.util.stream.Stream;

import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallTests;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.params.provider.Arguments;

@Tag("prc-calltests")
public class Tests extends PrecompileCallTests<CallParameters> {
  // Set sample size with potential for override.
  private static int ECRECOVER_SAMPLE_SIZE =
      Integer.parseInt(System.getenv().getOrDefault("PRC_CALLTESTS_SAMPLE_SIZE", "250"));

  public static Stream<Arguments> parameterGeneration() {
    System.out.println("ECRECOVER TESTS=" + ParameterGeneration.parameterGeneration().size());
    return randomSampleByDayOfMonth(
        ECRECOVER_SAMPLE_SIZE, ParameterGeneration.parameterGeneration())
        .stream();
  }
}

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

import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.GasParameter.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.GasParameter.PLENTY;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ReturnAtParameter.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing.MemoryContents.TOTAL_NUMBER_OF_PAIRS_OF_POINTS;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.opcode.OpCode.STATICCALL;

import java.util.ArrayList;
import java.util.List;

import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.GasParameter;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ReturnAtParameter;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.params.provider.Arguments;

public class ParameterGeneration {

  public static List<Arguments> parameterGeneration() {
    List<OpCode> CallOpCodes = List.of(CALL, STATICCALL);
    List<GasParameter> GasParameters = List.of(COST_MO, COST, PLENTY);
    List<ReturnAtParameter> ReturnAtParameters = List.of(PARTIAL, FULL);

    List<Arguments> argumentsList = new ArrayList<>();

    for (OpCode opCode : CallOpCodes) { // 4
      for (GasParameter gas : GasParameters) { // 3
        for (ReturnAtParameter returnAt : ReturnAtParameters) { // 2

          // empty call data cases
          argumentsList.add(
              Arguments.of(
                  new CallParameters(
                      opCode,
                      gas,
                      new MemoryContents(SmallPoint.INFINITY, LargePoint.INFINITY),
                      new CallDataRange(),
                      returnAt,
                      true)));

          argumentsList.add(
              Arguments.of(
                  new CallParameters(
                      opCode,
                      gas,
                      new MemoryContents(SmallPoint.INFINITY, LargePoint.INFINITY),
                      new CallDataRange(),
                      returnAt,
                      false)));
          for (int i = 0; i < TOTAL_NUMBER_OF_PAIRS_OF_POINTS; i++) {
            for (int j = i; j < TOTAL_NUMBER_OF_PAIRS_OF_POINTS; j++) {
              CallDataRange callDataRange = new CallDataRange(i, j);
              for (SmallPoint small : SmallPoint.values()) {
                for (LargePoint large : LargePoint.values()) {

                  MemoryContents memoryContent = new MemoryContents(small, large);

                  // nonempty call data cases
                  argumentsList.add(
                      Arguments.of(
                          new CallParameters(
                              opCode, gas, memoryContent, callDataRange, returnAt, true)));

                  argumentsList.add(
                      Arguments.of(
                          new CallParameters(
                              opCode, gas, memoryContent, callDataRange, returnAt, false)));
                }
              }
            }
          }
        }
      }
    }
    return argumentsList;
  }
}

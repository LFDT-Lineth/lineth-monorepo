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
package net.consensys.linea.zktracer.instructionprocessing.createTests.failure;

import static net.consensys.linea.zktracer.instructionprocessing.createTests.trivial.RootLevel.*;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class NoFailure extends TracerTestBase {

  /**
   * The following tests runs two separate CREATE2 deployments with different SALT parameters. No
   * address collision takes place, no failure condition F is raised.
   */
  @Test
  void noFailureConditionTest(TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(salt01)
        .push(0)
        .push(0)
        .push(1)
        .op(CREATE2)
        .push(salt02)
        .push(0)
        .push(0)
        .push(1)
        .op(CREATE2);

    run(program, chainConfig, testInfo);
  }

  /**
   * The following (1) precomputes a deployment address (2) stores it (3) transfers value to that
   * address (4) performs a deployment (CREATE2) at that address
   */
  @Test
  void noFailureConditionDespiteNonzeroBalanceTest(TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    precomputeDeploymentAddressOfEmptyInitCodeCreate2(program, salt01);
    storeAt(program, 0xadd7);
    program.push(0).push(0).push(0).push(0).push(1); // value
    loadFromStorage(program, 0xadd7); // address
    program.op(GAS).op(CALL);
    program.push(salt01).push(0).push(0).push(1).op(CREATE2);
    loadFromStorage(program, 0xadd7);
    program.op(EQ); // we expect the result to be true

    run(program, chainConfig, testInfo);
  }
}

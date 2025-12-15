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
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/**
 * The following tests raise the <b>Failure Condition F</b> with the <b>CREATE2</b> opcode. These
 * tests are sequential in nature: one <b>CREATE2</b> after another.
 */
@ExtendWith(UnitTestWatcher.class)
public class Create2InducedFailureTests extends TracerTestBase {

  @Test
  void failureConditionNonceTest(TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(salt01)
        .push(0)
        .push(0)
        .push(1)
        .op(CREATE2)
        .push(salt01)
        .push(0)
        .push(0)
        .push(1)
        .op(CREATE2);

    run(program, chainConfig, testInfo);
  }

  /**
   * We first produce init code that, when run, does
   *
   * <p><b>PUSH1 1</b>
   *
   * <p><b>PUSH1 0</b>
   *
   * <p><b>RETURN</b>
   *
   * <p>i.e. 0x60016000f3. We then (1) store that init code in memory (2) use it for a first CREATE2
   * which deploys a SMC with bytecode of length 1 equal to 0x00 (3) test the (EXT) code size, code
   * hash and balance of this new account (4) attempt a second deployment at that same exact
   * address, thus raising the <b>Failure Condition F</b>.
   */
  @Test
  void failureConditionNonceAndCodeTest(TestInfo testInfo) {

    BytecodeCompiler initCode = BytecodeCompiler.newProgram(chainConfig);
    initCode
        .push(1) // size
        .push(0)
        .op(RETURN);
    Bytes compiledInitCode = initCode.compile();

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(compiledInitCode)
        .push(8 * (32 - compiledInitCode.size()))
        .op(SHL)
        .push(0)
        .op(MSTORE); // this puts the init code in memory

    program
        .push(salt01)
        .push(compiledInitCode.size())
        .push(0)
        .push(1) // value
        .op(CREATE2); // first deployment, deploys monobyte byte code

    program.op(DUP1).op(DUP1).op(EXTCODEHASH).op(EXTCODESIZE).op(BALANCE);

    program
        .push(salt01)
        .push(compiledInitCode.size())
        .push(0)
        .push(1) // value
        .op(CREATE2); // second (attempted) deployment raises failure condition

    run(program, chainConfig, testInfo);
  }
}

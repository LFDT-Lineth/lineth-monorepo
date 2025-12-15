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
package net.consensys.linea.zktracer.instructionprocessing.callTests.abort;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.*;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/** CALL/ABORT's are revert sensitive. We test this with two CALL's. */
@ExtendWith(UnitTestWatcher.class)
public class MultiCallAbortTests extends TracerTestBase {

  @Test
  void normalCallThenAbortedCallToEoaThenRevert(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, CALL, 0, Address.fromHexString(eoaAddress), 1, 0, 0, 0, 0);
    appendInsufficientBalanceCall(
        program, CALL, 1000, Address.fromHexString(eoaAddress), 0, 0, 0, 0);
    program.push(6).push(7).op(REVERT);
    Bytes bytecode = program.compile();
    BytecodeRunner.of(bytecode).run(chainConfig, testInfo);
  }

  @Test
  void abortedCallNormalCallToEoaThenRevert(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendInsufficientBalanceCall(
        program, CALL, 1000, Address.fromHexString(eoaAddress), 0, 0, 0, 0);
    appendCall(program, CALL, 0, Address.fromHexString(eoaAddress), 1, 0, 0, 0, 0);
    program.push(6).push(7).op(REVERT);
    Bytes bytecode = program.compile();
    BytecodeRunner.of(bytecode).run(chainConfig, testInfo);
  }

  @Test
  void balanceThenAbortedCallToEoaThenRevert(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program.push(eoaAddress).op(BALANCE).op(POP);
    appendInsufficientBalanceCall(
        program, CALL, 1000, Address.fromHexString(eoaAddress), 0, 0, 0, 0);
    appendCall(program, CALL, 0, Address.fromHexString(eoaAddress), 0, 0, 0, 0, 0);
    program.push(6).push(7).op(REVERT);
    Bytes bytecode = program.compile();
    BytecodeRunner.of(bytecode).run(chainConfig, testInfo);
  }

  @Test
  void abortedCallThenBalanceToEoaThenRevert(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendInsufficientBalanceCall(
        program, CALL, 1000, Address.fromHexString(eoaAddress), 0, 0, 0, 0);
    appendCall(program, CALL, 0, Address.fromHexString(eoaAddress), 0, 0, 0, 0, 0);
    program.push(6).push(7).op(REVERT);
    Bytes bytecode = program.compile();
    BytecodeRunner.of(bytecode).run(chainConfig, testInfo);
  }
}

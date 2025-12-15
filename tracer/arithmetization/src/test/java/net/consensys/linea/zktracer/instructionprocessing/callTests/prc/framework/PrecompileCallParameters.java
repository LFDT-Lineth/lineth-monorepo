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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework;

import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.CodeExecutionMethods.memoryContentsHolderAddress1;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.CodeExecutionMethods.memoryContentsHolderAddress2;
import static net.consensys.linea.zktracer.opcode.OpCode.CALL;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.ChainConfig;
import org.hyperledger.besu.datatypes.Address;

public interface PrecompileCallParameters {

  boolean willRevert();

  PrecompileCallMemoryContents memoryContents();

  void appendCustomPrecompileCall(BytecodeCompiler program);

  /**
   * {@link #customPrecompileCallsSeparatedByReturnDataWipingOperation} constructs the byte code for
   * the <b>happy path</b> testing of the relevant <b>PRECOMPILE</b>.
   */
  default BytecodeCompiler customPrecompileCallsSeparatedByReturnDataWipingOperation(
      ChainConfig chainConfig) {

    // populate foreign accounts' byte code with call data
    this.memoryContents().setCodeOfHolderAccounts(chainConfig);

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    // populate memory with the data for first PRECOMPILE call
    copyForeignCodeToRam(program, memoryContentsHolderAddress1);

    // first PRECOMPILE call
    this.appendCustomPrecompileCall(program);
    copyHalfOfReturnDataOmittingTheFirstThirdOfIt(program, 0x2a);
    loadFirstReturnDataWordOntoStack(program, 0x02ff);

    // return data wiping
    appendInsufficientBalanceCall(
        program, CALL, 34_000, Address.fromHexString("b077c0ffee1337"), 13, 15, 17, 19);
    copyHalfOfReturnDataOmittingTheFirstThirdOfIt(program, 21);
    loadFirstReturnDataWordOntoStack(program, 48);

    // populate memory with the data for second PRECOMPILE call
    copyForeignCodeToRam(program, memoryContentsHolderAddress2);

    // second PRECOMPILE call
    this.appendCustomPrecompileCall(program);
    copyHalfOfReturnDataOmittingTheFirstThirdOfIt(program, 0x026c);
    loadFirstReturnDataWordOntoStack(program, 0x00);

    return program;
  }
}

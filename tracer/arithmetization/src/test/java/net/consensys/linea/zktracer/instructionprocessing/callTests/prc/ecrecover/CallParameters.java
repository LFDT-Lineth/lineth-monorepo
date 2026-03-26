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

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.GasParameter;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ReturnAtParameter;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallMemoryContents;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallParameters;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import org.hyperledger.besu.datatypes.Address;

public class CallParameters implements PrecompileCallParameters {

  public final OpCode call;
  public final GasParameter gas;
  public final MemoryContents memoryContents;
  public final CallDataSizeParameter cds;
  public final ReturnAtParameter returnAt;
  public final boolean willRevert;

  public CallParameters(
      OpCode call,
      GasParameter gas,
      MemoryContents memoryContent,
      CallDataSizeParameter cds,
      ReturnAtParameter returnAt,
      boolean willRevert) {
    this.call = call;
    this.gas = gas;
    this.memoryContents = memoryContent;
    this.cds = cds;
    this.returnAt = returnAt;
    this.willRevert = willRevert;
  }

  public boolean willRevert() {
    return willRevert;
  }

  public PrecompileCallMemoryContents memoryContents() {
    return memoryContents;
  }

  public void appendCustomPrecompileCall(BytecodeCompiler program) {
    OpCodeData callInfo = program.opCodeData(call);
    // push r@c onto the stack
    switch (returnAt) {
      case EMPTY -> program.push(0);
      case PARTIAL ->
          program.push(12 + 6); // the first 12 bytes are zeros for successful ECRECOVER calls
      case FULL -> program.push(WORD_SIZE);
    }

    // push the r@o onto the stack
    program.push(4 * WORD_SIZE);

    // push the cds onto the stack
    switch (cds) {
      case EMPTY -> program.push(0);
      case MISSING_FINAL_BYTE_OF_R -> program.push(3 * WORD_SIZE - 1);
      case MISSING_FINAL_BYTE_OF_S -> program.push(4 * WORD_SIZE - 1);
      case FULL -> program.op(MSIZE);
    }

    // push the cdo onto the stack;
    program.push(0);

    // if appropriate, push the value onto the stack
    if (callInfo.callHasValueArgument()) {
      program.push(0x0600);
    }

    program.push(Address.ECREC.getBytes());

    // push gas onto the stack
    int callStipend = callInfo.callHasValueArgument() ? 2_300 : 0;
    switch (gas) {
      case ZERO -> program.push(0); // interesting in the nonzero value case
      case COST_MO -> program.push(3000 - callStipend - 1);
      case COST -> program.push(3000 - callStipend);
      case PLENTY -> program.push(2).op(GAS).op(DIV); // half of gas
      default -> throw new RuntimeException("Unsupported gas parameter");
    }

    program.op(call);
  }

  @Override
  public String toString() {
    return "EcrecoverCallParameters{"
        + "call="
        + call
        + ", gas="
        + gas
        + ", memoryContents="
        + memoryContents
        + ", cds="
        + cds
        + ", returnAt="
        + returnAt
        + ", willRevert="
        + willRevert
        + '}';
  }
}

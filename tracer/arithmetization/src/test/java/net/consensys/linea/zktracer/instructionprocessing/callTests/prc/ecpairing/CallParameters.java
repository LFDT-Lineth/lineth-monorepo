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

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing.MemoryContents.SIZE_OF_PAIR_OF_POINTS;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing.MemoryContents.TOTAL_NUMBER_OF_PAIRS_OF_POINTS;
import static net.consensys.linea.zktracer.opcode.OpCode.DIV;
import static net.consensys.linea.zktracer.opcode.OpCode.GAS;

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
  public final CallDataRange callDataRange;
  public final ReturnAtParameter returnAt;
  public final boolean willRevert;

  public CallParameters(
      OpCode call,
      GasParameter gas,
      MemoryContents memoryContents,
      CallDataRange callDataRange,
      ReturnAtParameter returnAt,
      boolean willRevert) {
    this.call = call;
    this.gas = gas;
    this.memoryContents = memoryContents;
    this.callDataRange = callDataRange;
    this.returnAt = returnAt;
    this.willRevert = willRevert;
  }

  @Override
  public boolean willRevert() {
    return willRevert;
  }

  @Override
  public PrecompileCallMemoryContents memoryContents() {
    return memoryContents;
  }

  @Override
  public void appendCustomPrecompileCall(BytecodeCompiler program) {
    OpCodeData callInfo = program.opCodeData(call);
    // push r@c onto the stack
    switch (this.returnAt) {
      case EMPTY -> program.push(0);
      case PARTIAL -> program.push(13);
      case FULL -> program.push(WORD_SIZE);
      default -> throw new RuntimeException("Unsupported returnAt parameter");
    }

    // push the r@o onto the stack
    program.push(TOTAL_NUMBER_OF_PAIRS_OF_POINTS * SIZE_OF_PAIR_OF_POINTS);

    // push the cds onto the stack
    final int cds = callDataRange.numberOfPairsOfPoints() * SIZE_OF_PAIR_OF_POINTS;
    program.push(cds);

    // push the cdo onto the stack;
    final int cdo =
        callDataRange.isEmpty() ? 0 : callDataRange.firstPoint() * SIZE_OF_PAIR_OF_POINTS;
    program.push(cdo);

    // if appropriate, push the value onto the stack
    if (callInfo.callHasValueArgument()) {
      program.push(0x0a00);
    }

    program.push(Address.ALTBN128_MUL.getBytes());

    // push gas onto the stack
    int callStipend = callInfo.callHasValueArgument() ? 2_300 : 0;
    int prcCost =
        GAS_CONST_ECPAIRING + callDataRange.numberOfPairsOfPoints() * GAS_CONST_ECPAIRING_PAIR;
    switch (gas) {
      case ZERO -> program.push(0); // interesting in the nonzero value case
      case COST_MO -> program.push(prcCost - callStipend - 1);
      case COST -> program.push(prcCost - callStipend);
      case PLENTY -> program.push(2).op(GAS).op(DIV); // half of gas
      default -> throw new RuntimeException("Unsupported gas parameter");
    }

    program.op(call);
  }

  @Override
  public String toString() {
    return "CallParameters{"
        + "call="
        + call
        + ", gas="
        + gas
        + ", memoryContents="
        + memoryContents
        + ", callDataRange="
        + (callDataRange.isEmpty() ? "∅" : callDataRange)
        + ", returnAt="
        + returnAt
        + ", willRevert="
        + willRevert
        + '}';
  }
}

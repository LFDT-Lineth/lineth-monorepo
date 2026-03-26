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
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.RelativeRangePosition.OVERLAP;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.*;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallMemoryContents;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallParameters;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;

public class CallParameters implements PrecompileCallParameters {
  public final OpCode call;
  public GasParameter gas;
  public final HashPrecompile prc;
  public ValueParameter value;
  public final CallOffset cdo;
  public final CallSize cds;
  public final CallOffset rao;
  public final CallSize rac;
  public final MemoryContents memoryContents;
  public final RelativeRangePosition relPos;
  public final boolean willRevert;

  public CallParameters(
      OpCode call,
      GasParameter gas,
      HashPrecompile prc,
      ValueParameter value,
      CallOffset cdo,
      CallSize cds,
      CallOffset rao,
      CallSize rac,
      MemoryContents memoryContents,
      RelativeRangePosition relPos,
      boolean willRevert) {
    this.call = call;
    this.gas = gas;
    this.prc = prc;
    this.value = value;
    this.cdo = cdo;
    this.cds = cds;
    this.rao = rao;
    this.rac = rac;
    this.memoryContents = memoryContents;
    this.relPos = relPos;
    this.willRevert = willRevert;
  }

  public CallParameters next() {
    return new CallParameters(
        call, gas, prc.next(), value, cdo, cds, rao, rac, memoryContents, relPos, willRevert);
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
    // if DISJOINT the "return at range"; it lives among words 2 and 3 of RAM
    switch (rac) {
      case ZERO -> program.push(0);
      case WORD -> program.push(WORD_SIZE);
      case OTHER -> program.push(58);
    }

    switch (rao) {
      case ALIGNED -> program.push((relPos == OVERLAP ? 0 : 2 * WORD_SIZE) + prc.smallOffset1());
      case MISALIGNED ->
          program.push((relPos == OVERLAP ? 0 : 2 * WORD_SIZE) + 4 + prc.smallOffset1());
      case INFINITY -> program.push("ff".repeat(32));
    }

    // call data can occupy up to 52 bytes; it lives among words 0 and 1 of RAM
    int callDataSize =
        switch (cds) {
          case ZERO -> 0;
          case WORD -> WORD_SIZE;
          case OTHER -> 43;
        };
    program.push(callDataSize);

    switch (cdo) {
      case ALIGNED -> program.push(prc.smallOffset2());
      case MISALIGNED -> program.push(13 + prc.smallOffset2());
      case INFINITY -> program.push("ff".repeat(32));
    }

    if (callInfo.callHasValueArgument()) {
      switch (value) {
        case ZERO -> program.push(0);
        case ONE -> program.push(1);
        case VALUE -> program.push("69");
      }
    }

    // push address
    program.push(prc.getAddress().getBytes());

    // push gas parameter
    int cost = prc.cost(callDataSize);
    switch (gas) {
      case ZERO -> program.push(0);
      case COST_MO -> program.push(cost - 1);
      case COST -> program.push(cost);
      case PLENTY -> program.push(2).op(GAS).op(DIV); // half of gas
      case MAX -> program.push("ff".repeat(32));
    }

    program.op(call);
  }

  public final boolean willMxpx() {
    final boolean callDataMxpx = (cds != CallSize.ZERO) && (cdo == CallOffset.INFINITY);
    final boolean returnAtMxpx = (rac != CallSize.ZERO) && (rao == CallOffset.INFINITY);
    return callDataMxpx || returnAtMxpx;
  }

  @Override
  public String toString() {
    return "CallParameters{"
        + "call="
        + call
        + ", gas="
        + gas
        + ", prc="
        + prc
        + ", value="
        + value
        + ", cdo="
        + cdo
        + ", cds="
        + cds
        + ", rao="
        + rao
        + ", rac="
        + rac
        + ", memoryContents="
        + memoryContents
        + ", relPos="
        + relPos
        + ", willRevert="
        + willRevert
        + '}';
  }
}

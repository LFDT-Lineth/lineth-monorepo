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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.modexp;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.GasParameter;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.RelativeRangePosition;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ReturnAtParameter;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallMemoryContents;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallParameters;
import net.consensys.linea.zktracer.opcode.OpCode;

public class CallParameters implements PrecompileCallParameters {

  public final OpCode call;
  public final GasParameter gas;
  public final MemoryContents callData;
  public final ReturnAtParameter returnAt;
  public final RelativeRangePosition relPos;
  public final boolean willRevert;

  public CallParameters(
      OpCode call,
      GasParameter gas,
      MemoryContents memoryContents,
      ReturnAtParameter returnAt,
      RelativeRangePosition relPos,
      boolean willRevert) {
    this.call = call;
    this.gas = gas;
    this.callData = memoryContents;
    this.returnAt = returnAt;
    this.relPos = relPos;
    this.willRevert = willRevert;
  }

  @Override
  public boolean willRevert() {
    return willRevert;
  }

  @Override
  public PrecompileCallMemoryContents memoryContents() {
    return callData;
  }

  @Override
  public void appendCustomPrecompileCall(BytecodeCompiler program) {}
}

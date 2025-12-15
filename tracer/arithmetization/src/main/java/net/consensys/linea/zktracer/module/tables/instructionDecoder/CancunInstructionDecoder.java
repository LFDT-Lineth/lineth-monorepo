/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.tables.instructionDecoder;

import static net.consensys.linea.zktracer.opcode.InstructionFamily.MCOPY;
import static net.consensys.linea.zktracer.opcode.InstructionFamily.TRANSIENT;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.OpCodes;

public class CancunInstructionDecoder extends LondonInstructionDecoder {

  public CancunInstructionDecoder(OpCodes opCodes) {
    super(opCodes);
  }

  @Override
  protected void traceTransientFamily(OpCodeData op, Trace.Instdecoder trace) {
    trace.familyTransient(op.instructionFamily() == TRANSIENT);
  }

  @Override
  protected void traceMcopyFamily(OpCodeData op, Trace.Instdecoder trace) {
    trace.familyMcopy(op.instructionFamily() == MCOPY);
  }

  @Override
  protected void traceMxpFlag(OpCodeData op, Trace.Instdecoder trace) {
    // From Cancun, we have a Mxp flag available
    trace.mxpFlag(op.isMxp());
  }

  @Override
  protected void traceMxpScenario(OpCodeData op, Trace.Instdecoder trace) {
    trace
        .isMsize(op.isMSize())
        .isReturn(op.isReturn())
        .isMcopy(op.isMCopy())
        .isFixedSize1(op.isFixedSize1())
        .isFixedSize32(op.isFixedSize32())
        .isSingleMaxOffset(op.isSingleOffset())
        .isDoubleMaxOffset(op.isDoubleOffset())
        .isWordPricing(op.isWordPricing())
        .isBytePricing(op.isBytePricing());
  }
}

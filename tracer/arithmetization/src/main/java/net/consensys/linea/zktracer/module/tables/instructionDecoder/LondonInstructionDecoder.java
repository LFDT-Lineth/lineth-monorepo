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

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.OpCodes;
import net.consensys.linea.zktracer.opcode.gas.MxpType;

public class LondonInstructionDecoder extends InstructionDecoder {

  public LondonInstructionDecoder(OpCodes opCodes) {
    super(opCodes);
  }

  @Override
  protected void traceTransientFamily(OpCodeData op, Trace.Instdecoder trace) {
    // London does not have any transient family opcodes, they appear in Cancun
  }

  @Override
  protected void traceMcopyFamily(OpCodeData op, Trace.Instdecoder trace) {
    // London does not have any mcopy family opcodes, they appear in Cancun
  }

  @Override
  protected void traceMxpFlag(OpCodeData op, Trace.Instdecoder trace) {
    // mxp flag is determined with types label in London
    trace.mxpFlag(op.isMxpLondon());
  }

  @Override
  protected void traceMxpScenario(OpCodeData op, Trace.Instdecoder trace) {
    trace
        .mxpType1(op.billing().type() == MxpType.TYPE_1)
        .mxpType2(op.billing().type() == MxpType.TYPE_2)
        .mxpType3(op.billing().type() == MxpType.TYPE_3)
        .mxpType4(op.billing().type() == MxpType.TYPE_4)
        .mxpType5(op.billing().type() == MxpType.TYPE_5);
  }
}

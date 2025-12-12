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

package net.consensys.linea.zktracer.module.hub.fragment.stack;

import static net.consensys.linea.zktracer.opcode.InstructionFamily.MCOPY;
import static net.consensys.linea.zktracer.opcode.InstructionFamily.TRANSIENT;

import java.util.List;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.common.CommonFragmentValues;
import net.consensys.linea.zktracer.module.hub.signals.AbortingConditions;
import net.consensys.linea.zktracer.opcode.InstructionFamily;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.gas.projector.GasProjection;
import net.consensys.linea.zktracer.runtime.stack.Stack;
import net.consensys.linea.zktracer.runtime.stack.StackItem;

public class CancunStackFragment extends LondonStackFragment {
  public CancunStackFragment(
      Hub hub,
      Stack stack,
      List<StackItem> stackOps,
      short exceptions,
      AbortingConditions aborts,
      GasProjection gp,
      boolean isDeploying,
      CommonFragmentValues commonFragmentValues) {
    super(hub, stack, stackOps, exceptions, aborts, gp, isDeploying, commonFragmentValues);
  }

  @Override
  protected void traceMcopyFamily(Trace.Hub trace, InstructionFamily currentInstFamily) {
    trace.pStackMcopyFlag(currentInstFamily == MCOPY);
  }

  @Override
  protected void traceTransientFamily(Trace.Hub trace, InstructionFamily currentInstFamily) {
    trace.pStackTransFlag(currentInstFamily == TRANSIENT);
  }

  @Override
  protected void traceMxpFlag(Trace.Hub trace, OpCodeData opCodeData) {
    trace.pStackMxpFlag(opCodeData.isMxp());
  }
}

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

package net.consensys.linea.zktracer.module.hub.section;

import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.opcode.OpCodeData;

public class StackOnlySection extends TraceSection {
  public static final short NB_ROWS_HUB_SIMPLE_STACK_OP = 1;

  public StackOnlySection(Hub hub, OpCodeData opcode) {
    super(hub, NB_ROWS_HUB_SIMPLE_STACK_OP);

    this.addStack(hub);

    if (Exceptions.none(hub.pch().exceptions())) {
      switch (opcode.instructionFamily()) {
        case ADD -> hub.add().callAdd(hub.messageFrame(), opcode.mnemonic());
        // case BIN -> hub.bin().callBin(hub.messageFrame(), opcode.mnemonic()); done by corset
        case MOD -> hub.mod().callMod(hub.messageFrame(), opcode.mnemonic());
        case SHF -> hub.shf().callShf(hub.messageFrame(), opcode.mnemonic());
        case WCP -> hub.wcp().callWcp(hub.messageFrame(), opcode.mnemonic());
        case EXT -> hub.ext().callExt(hub.messageFrame(), opcode.mnemonic());
        case MUL -> hub.mul().callMul(hub.messageFrame(), opcode.mnemonic());
        case BATCH -> {
          if (opcode.mnemonic() == BLOCKHASH) {
            hub.blockhash().callBlockhash(hub.messageFrame());
          }
        }
        default -> {}
      }
    }
  }
}

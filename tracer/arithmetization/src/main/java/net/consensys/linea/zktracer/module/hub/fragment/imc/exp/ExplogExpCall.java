/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.module.hub.fragment.imc.exp;

import static net.consensys.linea.zktracer.Trace.EXP_INST_EXPLOG;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_EXP_BYTE;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Accessors(fluent = true)
@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class ExplogExpCall implements ExpCall {
  @EqualsAndHashCode.Include final EWord exponent;
  final long dynCost;

  public ExplogExpCall(MessageFrame frame) {
    this.exponent = EWord.of(frame.getStackItem(1));
    this.dynCost = (long) GAS_CONST_G_EXP_BYTE * exponent.byteLength();
  }

  @Override
  public int expInstruction() {
    return EXP_INST_EXPLOG;
  }

  @Override
  public int compareTo(ExpCall op2) {
    final ExplogExpCall o2 = (ExplogExpCall) op2;

    final int dynCostComp = Long.compare(dynCost, o2.dynCost());
    if (dynCostComp != 0) {
      return dynCostComp;
    }
    return exponent.compareTo(o2.exponent());
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscExpFlag(true)
        .pMiscExpInst(EXP_INST_EXPLOG)
        .pMiscExpData1(exponent.hi())
        .pMiscExpData2(exponent.lo())
        .pMiscExpData5(Bytes.ofUnsignedLong(dynCost));
  }

  public String toString() {
    return "EXPLOG(" + exponent.toString() + ")=" + dynCost;
  }
}

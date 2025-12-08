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

package net.consensys.linea.zktracer.module.stp;

import static com.google.common.base.Preconditions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.hub.fragment.imc.StpCall;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;

@Accessors(fluent = true)
@Getter
@EqualsAndHashCode(callSuper = false)
public final class StpOperation extends ModuleOperation {

  public static final short NB_ROWS_STP = 1;

  private final StpCall stpCall;

  public StpOperation(StpCall stpCall) {
    this.stpCall = stpCall;
  }

  long getGDiff() {
    checkArgument(
        !stpCall.outOfGasException(),
        "STP: attempting to compute gDiff = gasActual - gasUpfront, yet gasActual = %s < %s =gasUpfront",
        stpCall.gasActual(),
        stpCall.upfrontGasCost());
    return stpCall.gasActual() - stpCall.upfrontGasCost();
  }

  long getGDiffOver64() {
    return this.getGDiff() / 64;
  }

  long get63of64GDiff() {
    return this.getGDiff() - this.getGDiffOver64();
  }

  void trace(Trace.Stp trace) {
    long gasOopkt;
    // Determine out-of-pocket value
    if (stpCall.opCodeData().isCreate()) {
      gasOopkt = stpCall.outOfGasException() ? 0 : this.get63of64GDiff();
    } else {
      gasOopkt = stpCall.gasPaidOutOfPocket();
    }
    //
    trace
        .inst(UnsignedByte.of(stpCall.opCode().byteValue()))
        .value(stpCall.value())
        .oogx(stpCall.outOfGasException())
        .gas(stpCall.gas())
        .gasActual(Bytes.ofUnsignedLong(stpCall.gasActual()))
        .gasMxp(Bytes.ofUnsignedLong(stpCall.memoryExpansionGas()))
        .gasUpfront(Bytes.ofUnsignedLong(stpCall.upfrontGasCost()))
        .gasOop(Bytes.ofUnsignedLong(gasOopkt))
        .gasStipend(Bytes.ofUnsignedLong(stpCall.stipend()))
        .exists(stpCall.exists())
        .warm(stpCall.warm())
        .validateRow();
  }

  @Override
  protected int computeLineCount() {
    return NB_ROWS_STP;
  }
}

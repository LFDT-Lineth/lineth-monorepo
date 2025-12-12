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

package net.consensys.linea.zktracer.module.mxp.moduleCall;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.opcode.gas.MxpType;
import org.apache.tuweni.bytes.Bytes;

/** The parent class of this MXP Call is located in the Hub. */
public class LondonMxpCall extends MxpCall {

  public LondonMxpCall(Hub hub) {
    super(hub);
  }

  public void setMayTriggerNontrivialMmuOperation() {
    MxpType mxpType = getOpCodeData().billing().type();
    this.mayTriggerNontrivialMmuOperation =
        mxpType == MxpType.TYPE_4 && !mxpx && getSize1().loBigInt().signum() > 0;
  }

  public void traceMayTriggerNonTrivialMmuOperationFromMxpx(Trace.Hub trace) {
    trace.pMiscMxpMtntop(this.mayTriggerNontrivialMmuOperation);
  }

  public void traceMxpWords(Trace.Hub trace) {
    trace.pMiscMxpWords(Bytes.ofUnsignedLong(this.memorySizeInWords));
  }
}

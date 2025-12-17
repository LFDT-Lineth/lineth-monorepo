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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.create;

import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_CREATE;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction.MAX_INIT_CODE_SIZE_BYTES;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class ShanghaiCreateOobCall extends LondonCreateOobCall {

  public ShanghaiCreateOobCall() {
    super();
  }

  @Override
  public int ctMax() {
    return CT_MAX_CREATE;
  }

  protected void codeSizeSnapshot(final MessageFrame frame) {
    setCodeSize(clampedToLong(frame.getStackItem(2)));
  }

  protected void traceOobData10column(Trace.Oob trace, long codeSize) {
    trace.data10(Bytes.ofUnsignedLong(codeSize));
  }

  protected void traceHubData10column(Trace.Hub trace, long codeSize) {
    trace.pMiscOobData10(Bytes.ofUnsignedLong(codeSize));
  }

  protected OobExoCall exceedsMaxInitCodeSize(Wcp wcp) {
    return callToLT(wcp, MAX_INIT_CODE_SIZE_BYTES, Bytes.ofUnsignedInt(codeSize));
  }
}

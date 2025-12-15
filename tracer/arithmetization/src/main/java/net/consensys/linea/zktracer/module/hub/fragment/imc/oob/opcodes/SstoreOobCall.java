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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes;

import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_CALL_STIPEND;
import static net.consensys.linea.zktracer.Trace.OOB_INST_SSTORE;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_SSTORE;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.types.Conversions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class SstoreOobCall extends OobCall {

  public static final short NB_ROWS_OOB_SSTORE = CT_MAX_SSTORE + 1;

  private static final Bytes GAS_CONST_G_CALL_STIPEND_BYTES =
      Bytes.minimalBytes(GAS_CONST_G_CALL_STIPEND);

  // Inputs
  @EqualsAndHashCode.Include Bytes gas;

  // Outputs
  boolean sstorex;

  public SstoreOobCall() {
    super();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    setGas(Bytes.minimalBytes(frame.getRemainingGas()));
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall sufficientGasCall = callToLT(wcp, GAS_CONST_G_CALL_STIPEND_BYTES, gas);
    exoCalls.add(sufficientGasCall);
    final boolean sufficientGas = bytesToBoolean(sufficientGasCall.result());
    setSstorex(!sufficientGas);
  }

  @Override
  public int ctMax() {
    return CT_MAX_SSTORE;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace.isSstore(true).oobInst(OOB_INST_SSTORE).data5(gas).data7(booleanToBytes(sstorex));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_SSTORE)
        .pMiscOobData5(gas)
        .pMiscOobData7(booleanToBytes(sstorex));
  }
}

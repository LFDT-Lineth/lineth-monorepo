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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles;

import static net.consensys.linea.zktracer.Trace.OOB_INST_BLAKE_CDS;
import static net.consensys.linea.zktracer.types.Conversions.*;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class Blake2fCallDataSizeOobCall extends OobCall {
  public static final BigInteger BLAKE_VALID_CDS_BI = BigInteger.valueOf(213);
  // Inputs
  @EqualsAndHashCode.Include EWord cds;
  @EqualsAndHashCode.Include EWord returnAtCapacity;

  // Outputs
  boolean hubSuccess;
  boolean returnAtCapacityNonZero;

  public Blake2fCallDataSizeOobCall() {
    super();
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    final OpCodeData opCode = hub.opCodeData(frame);
    final EWord cds = EWord.of(frame.getStackItem(opCode.callCdsStackIndex()));
    final EWord returnAtCapacity =
        EWord.of(frame.getStackItem(opCode.callReturnAtCapacityStackIndex()));
    setCds(cds);
    setReturnAtCapacity(returnAtCapacity);
  }

  @Override
  public void setOutputs() {
    setHubSuccess(cds.toUnsignedBigInteger().equals(BLAKE_VALID_CDS_BI));
    setReturnAtCapacityNonZero(!returnAtCapacity.isZero());
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_BLAKE_CDS)
        .data2(cds.trimLeadingZeros())
        .data3(returnAtCapacity.trimLeadingZeros())
        .data4(booleanToBytes(hubSuccess))
        .data8(booleanToBytes(returnAtCapacityNonZero))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_BLAKE_CDS)
        .pMiscOobData2(cds.trimLeadingZeros())
        .pMiscOobData3(returnAtCapacity.trimLeadingZeros())
        .pMiscOobData4(booleanToBytes(hubSuccess))
        .pMiscOobData8(booleanToBytes(returnAtCapacityNonZero));
  }
}

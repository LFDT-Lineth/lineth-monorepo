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
package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.pricingOobCall;

import static net.consensys.linea.zktracer.Trace.OOB_INST_MODEXP_PRICING;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_MODEXP_PRICING;
import static net.consensys.linea.zktracer.module.oob.OobOperation.computeExponentLog;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.booleanToBytes;
import static org.hyperledger.besu.evm.internal.Words.clampedToInt;

import java.math.BigInteger;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata.ModexpMetadata;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public abstract class ModexpPricingOobCall extends OobCall {
  public static final short NB_ROWS_OOB_MODEXP_PRICING = CT_MAX_MODEXP_PRICING + 1;

  // Inputs
  @EqualsAndHashCode.Include final ModexpMetadata metadata;
  @EqualsAndHashCode.Include final Bytes callGas;
  @EqualsAndHashCode.Include EWord returnAtCapacity;
  @EqualsAndHashCode.Include BigInteger exponentLog;

  // Outputs
  boolean ramSuccess;
  int maxMbsBbs;
  BigInteger returnGas;
  boolean returnAtCapacityNonZero;

  public ModexpPricingOobCall(ModexpMetadata metadata, long calleeGas) {
    super();
    this.metadata = metadata;
    this.callGas = Bytes.ofUnsignedLong(calleeGas);
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    final OpCodeData opCode = hub.opCodeData(frame);
    setReturnAtCapacity(EWord.of(frame.getStackItem(opCode.callReturnAtCapacityStackIndex())));

    final int cds = clampedToInt(frame.getStackItem(opCode.callCdsStackIndex()));
    setExponentLog(BigInteger.valueOf(computeExponentLog(metadata, cds)));
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isModexpPricing(true)
        .oobInst(OOB_INST_MODEXP_PRICING)
        .data1(callGas)
        .data3(returnAtCapacity.trimLeadingZeros())
        .data4(booleanToBytes(ramSuccess))
        .data5(bigIntegerToBytes(returnGas))
        .data6(bigIntegerToBytes(exponentLog))
        .data7(Bytes.ofUnsignedInt(maxMbsBbs))
        .data8(booleanToBytes(returnAtCapacityNonZero));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_MODEXP_PRICING)
        .pMiscOobData1(callGas)
        .pMiscOobData3(returnAtCapacity.trimLeadingZeros())
        .pMiscOobData4(booleanToBytes(ramSuccess))
        .pMiscOobData5(bigIntegerToBytes(returnGas))
        .pMiscOobData6(bigIntegerToBytes(exponentLog))
        .pMiscOobData7(Bytes.ofUnsignedInt(maxMbsBbs))
        .pMiscOobData8(booleanToBytes(returnAtCapacityNonZero));
  }

  @Override
  public int ctMax() {
    return CT_MAX_MODEXP_PRICING;
  }
}

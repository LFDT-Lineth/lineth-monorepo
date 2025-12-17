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

import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_MEMORY;
import static net.consensys.linea.zktracer.Trace.Mxp.CT_MAX_UPDT_B;
import static net.consensys.linea.zktracer.module.mxp.MxpOperation.MXP_FROM_CTMAX_TO_LINECOUNT;
import static net.consensys.linea.zktracer.types.Conversions.*;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.mxp.MxpExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.internal.Words;

public abstract class CancunStateUpdateMxpCall extends CancunNotMSizeNorTrivialMxpCall {

  public static final short NB_ROWS_MXP_UPDT_B =
      (short) (CT_MAX_UPDT_B + MXP_FROM_CTMAX_TO_LINECOUNT);

  public CancunStateUpdateMxpCall(Hub hub) {
    super(hub);
    computeStateUpdt(hub.wcp(), hub.euc());
  }

  public void computeStateUpdt(Wcp wcp, Euc euc) {

    // We compute and assign the computation's result for each row

    // Row i + 7
    // Compute useParams1 and useParams2
    final BigInteger max1 =
        this.size1IsZero
            ? BigInteger.ZERO
            : this.offset1.toUnsignedBigInteger().add(this.size1.toUnsignedBigInteger());
    final BigInteger max2 =
        this.size2IsZero
            ? BigInteger.ZERO
            : this.offset2.toUnsignedBigInteger().add(this.size2.toUnsignedBigInteger());
    final BigInteger doubleOffset = booleanToBigInteger(this.opCodeData.isDoubleOffset());
    exoCalls[6] =
        MxpExoCall.callToLT(
            wcp,
            bigIntegerToBytes(max1.multiply(doubleOffset)),
            bigIntegerToBytes(max2.multiply(doubleOffset)));
    boolean useParams2 = bytesToBoolean(exoCalls[6].resultA()); // result of row i + 7
    boolean useParams1 = !useParams2;

    // Row i + 8
    // Compute floor and EYPa
    final BigInteger maxOffset1 =
        this.offset1
            .lo()
            .toUnsignedBigInteger()
            .add(this.size1.lo().toUnsignedBigInteger())
            .subtract(BigInteger.ONE);
    final BigInteger maxOffset2 =
        this.offset2
            .lo()
            .toUnsignedBigInteger()
            .add(this.size2.lo().toUnsignedBigInteger())
            .subtract(BigInteger.ONE);
    ;
    final BigInteger maxOffset =
        booleanToBigInteger(useParams1)
            .multiply(maxOffset1)
            .add(booleanToBigInteger(useParams2).multiply(maxOffset2));
    exoCalls[7] = MxpExoCall.callToEUC(euc, bigIntegerToBytes(maxOffset), unsignedIntToBytes(32));
    final Bytes floor = exoCalls[7].resultA();
    final BigInteger EYPa = floor.toUnsignedBigInteger().add(BigInteger.ONE);

    // row i + 9
    // Compute cMemQuadPart
    exoCalls[8] =
        MxpExoCall.callToEUC(euc, bigIntegerToBytes(EYPa.multiply(EYPa)), unsignedIntToBytes(512));
    final Bytes cMemQuadPart = exoCalls[8].resultA();

    // row i + 10
    // Compute updateInternalState
    exoCalls[9] = MxpExoCall.callToLT(wcp, longToBytes(words), bigIntegerToBytes(EYPa));
    final boolean updateInternalState = bytesToBoolean(exoCalls[9].resultA());

    // Determine state update
    final Bytes cMemLinearPart =
        bigIntegerToBytes(EYPa.multiply(BigInteger.valueOf(GAS_CONST_G_MEMORY)));
    this.isStateUpdate = updateInternalState;
    this.wordsNew = updateInternalState ? EYPa.longValue() : this.words;
    this.cMemNew =
        updateInternalState
            ? Words.clampedToLong(
                bigIntegerToBytes(
                    cMemQuadPart.toUnsignedBigInteger().add(cMemLinearPart.toUnsignedBigInteger())))
            : this.cMem;
  }

  // We set ctMax to the minimum number of rows required for the following scenarii (State update
  // byte and State update word pricing)
  @Override
  public int ctMax() {
    return CT_MAX_UPDT_B;
  }
}

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

package net.consensys.linea.zktracer.module.oob;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.opcode.OpCode.ADD;
import static net.consensys.linea.zktracer.types.Conversions.*;

import java.math.BigInteger;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.tables.bls.BlsRt;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

@Builder
@Getter
@Accessors(fluent = true)
public class OobExoCall {

  @Builder.Default private final boolean addFlag = false;
  @Builder.Default private final boolean modFlag = false;
  @Builder.Default private final boolean wcpFlag = false;
  @Builder.Default private final boolean blsRtFlag = false;
  @Builder.Default private final int instruction = 0;
  @Builder.Default private final Bytes32 arg1 = Bytes32.ZERO;
  @Builder.Default private final Bytes32 arg2 = Bytes32.ZERO;
  @Builder.Default private final Bytes result = ZERO;

  protected void trace(Trace.Oob trace) {
    trace
        .addFlag(addFlag)
        .modFlag(modFlag)
        .wcpFlag(wcpFlag)
        .outgoingInst(instruction)
        .outgoingData1(arg1.slice(0, LLARGE))
        .outgoingData2(arg1.slice(LLARGE, LLARGE))
        .outgoingData3(arg2.slice(0, LLARGE))
        .outgoingData4(arg2.slice(LLARGE, LLARGE))
        .outgoingResLo(addFlag ? ZERO : result);
    // Prague
    if (blsRtFlag) {
      trace.blsRefTableFlag(true);
    }
  }

  public static OobExoCall callToADD(final Add add, final Bytes arg1, final Bytes arg2) {
    final Bytes32 arg1B32 = Bytes32.leftPad(arg1);
    final Bytes32 arg2B32 = Bytes32.leftPad(arg2);

    return OobExoCall.builder()
        .addFlag(true)
        .instruction(EVM_INST_ADD)
        .arg1(arg1B32)
        .arg2(arg2B32)
        .result(bigIntegerToBytes(add.call(ADD, arg1B32, arg2B32)))
        .build();
  }

  public static OobExoCall callToLT(final Wcp wcp, Bytes arg1, Bytes arg2) {

    final Bytes32 arg1B32 = Bytes32.leftPad(arg1);
    final Bytes32 arg2B32 = Bytes32.leftPad(arg2);

    return OobExoCall.builder()
        .wcpFlag(true)
        .instruction(EVM_INST_LT)
        .arg1(arg1B32)
        .arg2(arg2B32)
        .result(booleanToBytes(wcp.callLT(arg1B32, arg2B32)))
        .build();
  }

  public static OobExoCall callToGT(final Wcp wcp, Bytes arg1, Bytes arg2) {

    final Bytes32 arg1B32 = Bytes32.leftPad(arg1);
    final Bytes32 arg2B32 = Bytes32.leftPad(arg2);

    return OobExoCall.builder()
        .wcpFlag(true)
        .instruction(EVM_INST_GT)
        .arg1(arg1B32)
        .arg2(arg2B32)
        .result(booleanToBytes(wcp.callGT(arg1B32, arg2B32)))
        .build();
  }

  public static OobExoCall callToEQ(final Wcp wcp, Bytes arg1, Bytes arg2) {

    final Bytes32 arg1B32 = Bytes32.leftPad(arg1);
    final Bytes32 arg2B32 = Bytes32.leftPad(arg2);

    return OobExoCall.builder()
        .wcpFlag(true)
        .instruction(EVM_INST_EQ)
        .arg1(arg1B32)
        .arg2(arg2B32)
        .result(booleanToBytes(wcp.callEQ(arg1B32, arg2B32)))
        .build();
  }

  public static OobExoCall callToIsZero(final Wcp wcp, Bytes arg1) {

    final Bytes32 arg1B32 = Bytes32.leftPad(arg1);

    return OobExoCall.builder()
        .wcpFlag(true)
        .instruction(EVM_INST_ISZERO)
        .arg1(arg1B32)
        .result(booleanToBytes(wcp.callISZERO(arg1B32)))
        .build();
  }

  public static OobExoCall noCall() {
    return OobExoCall.builder().build();
  }

  public static OobExoCall callToDIV(Mod mod, Bytes arg1, Bytes arg2) {

    final Bytes32 arg1B32 = Bytes32.leftPad(arg1);
    final Bytes32 arg2B32 = Bytes32.leftPad(arg2);

    return OobExoCall.builder()
        .modFlag(true)
        .instruction(EVM_INST_DIV)
        .arg1(arg1B32)
        .arg2(arg2B32)
        .result(bigIntegerToBytes(mod.callDIV(arg1B32, arg2B32)))
        .build();
  }

  public static OobExoCall callToMOD(Mod mod, Bytes arg1, Bytes arg2) {

    final Bytes32 arg1B32 = Bytes32.leftPad(arg1);
    final Bytes32 arg2B32 = Bytes32.leftPad(arg2);

    return OobExoCall.builder()
        .modFlag(true)
        .instruction(EVM_INST_MOD)
        .arg1(arg1B32)
        .arg2(arg2B32)
        .result(bigIntegerToBytes(mod.callMOD(arg1B32, arg2B32)))
        .build();
  }

  public static OobExoCall callToBlsRefTable(final int instruction, final int numInputs) {
    return OobExoCall.builder()
        .blsRtFlag(true)
        .instruction(instruction)
        .arg1(EWord.of(BigInteger.valueOf(numInputs), BigInteger.ZERO).toBytes())
        .result(Bytes.ofUnsignedInt(BlsRt.getMsmDiscount(instruction, numInputs)))
        .build();
  }
}

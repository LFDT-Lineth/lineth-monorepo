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

package net.consensys.linea.zktracer.module.mmu.values;

import static net.consensys.linea.zktracer.types.Utils.leftPadToBytes16;

import java.math.BigInteger;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import org.apache.tuweni.bytes.Bytes;

@Getter
@Accessors(fluent = true)
public class HubToMmuValues {
  private final int mmuInstruction;
  @Setter private long sourceId;
  @Setter private long targetId;
  private final int auxId;
  private final BigInteger sourceOffsetHi;
  private final BigInteger sourceOffsetLo;
  private final long targetOffset;
  private final long size;
  private final long referenceOffset;
  private final long referenceSize;
  private final boolean successBit;
  private final Bytes limb1;
  private final Bytes limb2;
  private final int phase;
  private final int exoSum;
  private final boolean exoIsRom;
  private final boolean exoIsBlake2fModexp;
  private final boolean exoIsEcData;
  private final boolean exoIsBls;
  private final boolean exoIsRipSha;
  private final boolean exoIsKeccak;
  private final boolean exoIsLog;
  private final boolean exoIsTxcd;

  private HubToMmuValues(
      final MmuCall mmuCall, final boolean exoIsSource, final boolean exoIsTarget) {
    this.mmuInstruction = mmuCall.instruction();
    this.exoSum = mmuCall.exoSum();
    this.exoIsRom = mmuCall.exoIsRom();
    this.exoIsBlake2fModexp = mmuCall.exoIsBlakeModexp();
    this.exoIsEcData = mmuCall.exoIsEcData();
    this.exoIsBls = mmuCall.exoIsBlsData();
    this.exoIsRipSha = mmuCall.exoIsRipSha();
    this.exoIsKeccak = mmuCall.exoIsKec();
    this.exoIsLog = mmuCall.exoIsLog();
    this.exoIsTxcd = mmuCall.exoIsRlpTxn();
    this.sourceId = exoIsRom && exoIsSource ? -1 : mmuCall.sourceId();
    this.targetId = exoIsRom && exoIsTarget ? -1 : mmuCall.targetId();
    this.auxId = mmuCall.auxId();
    this.sourceOffsetHi = mmuCall.sourceOffset().hiBigInt();
    this.sourceOffsetLo = mmuCall.sourceOffset().loBigInt();
    this.targetOffset = mmuCall.targetOffset().toLong();
    this.size = mmuCall.size();
    this.referenceOffset = mmuCall.referenceOffset();
    this.referenceSize = mmuCall.referenceSize();
    this.successBit = mmuCall.successBit();
    this.limb1 = leftPadToBytes16(mmuCall.limb1());
    this.limb2 = leftPadToBytes16(mmuCall.limb2());
    this.phase = (int) mmuCall.phase();
  }

  public static HubToMmuValues fromMmuCall(
      final MmuCall mmuCall, final boolean exoIsSource, final boolean exoIsTarget) {
    return new HubToMmuValues(mmuCall, exoIsSource, exoIsTarget);
  }
}

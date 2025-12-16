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

package net.consensys.linea.zktracer.module.shakiradata;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.PHASE_KECCAK_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_KECCAK_RESULT;
import static net.consensys.linea.zktracer.Trace.PHASE_RIPEMD_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_RIPEMD_RESULT;
import static net.consensys.linea.zktracer.Trace.PHASE_SHA2_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_SHA2_RESULT;
import static net.consensys.linea.zktracer.Trace.Shakiradata.INDEX_MAX_RESULT;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.module.hub.Hub.newIdentifierFromStamp;
import static net.consensys.linea.zktracer.module.shakiradata.HashFunction.KECCAK;
import static net.consensys.linea.zktracer.module.shakiradata.HashFunction.RIPEMD;
import static net.consensys.linea.zktracer.module.shakiradata.HashFunction.SHA256;
import static net.consensys.linea.zktracer.types.Conversions.bytesToHex;
import static net.consensys.linea.zktracer.types.Utils.fromDataSizeToLimbCtMax;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;
import static org.hyperledger.besu.crypto.Hash.keccak256;

import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

@Accessors(fluent = true)
public class ShakiraDataOperation extends ModuleOperation {

  public static final short NB_ROWS_SHAKIRA_RESULT = INDEX_MAX_RESULT + 1;

  @Getter private final HashFunction hashType;
  private final Bytes hashInput;
  @Getter private final long ID;
  @Getter private final int inputSize;
  @Getter private final short lastNBytes;
  private final int indexMaxData;
  @Getter private Bytes32 result;

  /**
   * This constructor is used ONLY when we want to KECCAK the input, and it's not trivial to get the
   * result from Besu. Prefer the other constructor when we give the hash result
   */
  public ShakiraDataOperation(final int hubStamp, final Bytes input) {
    final Bytes32 hash = keccak256(input);

    hashType = KECCAK;
    ID = newIdentifierFromStamp(hubStamp);
    hashInput = input;
    inputSize = input.size();
    lastNBytes = (short) (inputSize % LLARGE == 0 ? LLARGE : inputSize % LLARGE);
    indexMaxData = fromDataSizeToLimbCtMax(inputSize);
    result = Bytes32.leftPad(hash);
  }

  public ShakiraDataOperation(
      final int hubStamp, final HashFunction hashFunction, final Bytes input, final Bytes hash) {
    hashType = hashFunction;
    ID = newIdentifierFromStamp(hubStamp);
    hashInput = input;
    inputSize = input.size();
    lastNBytes = (short) (inputSize % LLARGE == 0 ? LLARGE : inputSize % LLARGE);
    // this.indexMaxData = Math.ceilDiv(inputSize, LLARGE) - 1;
    indexMaxData = fromDataSizeToLimbCtMax(inputSize);
    result = Bytes32.leftPad(hash);
  }

  @Override
  protected int computeLineCount() {
    return indexMaxData + 1 + NB_ROWS_SHAKIRA_RESULT;
  }

  void trace(Trace.Shakiradata trace, final int stamp) {
    traceData(trace, stamp);
    traceResult(trace, stamp);
  }

  private void traceData(Trace.Shakiradata trace, final int stamp) {
    final boolean isShaData = hashType == SHA256;
    final boolean isKecData = hashType == KECCAK;
    final boolean isRipData = hashType == RIPEMD;
    final UnsignedByte phase =
        switch (hashType) {
          case SHA256 -> UnsignedByte.of(PHASE_SHA2_DATA);
          case KECCAK -> UnsignedByte.of(PHASE_KECCAK_DATA);
          case RIPEMD -> UnsignedByte.of(PHASE_RIPEMD_DATA);
        };

    for (int ct = 0; ct <= indexMaxData; ct++) {
      final boolean lastDataRow = ct == indexMaxData;
      trace
          .shakiraStamp(stamp)
          .id(ID)
          .phase(phase)
          .index(ct)
          .indexMax(indexMaxData)
          .limb(
              lastDataRow
                  ? rightPadTo(hashInput.slice(ct * LLARGE), LLARGE)
                  : hashInput.slice(ct * LLARGE, LLARGE))
          .nBytes(lastDataRow ? lastNBytes : LLARGE)
          .nBytesAcc(lastDataRow ? inputSize : (long) LLARGE * (ct + 1))
          .totalSize(inputSize)
          .isSha2Data(isShaData)
          .isKeccakData(isKecData)
          .isRipemdData(isRipData)
          .isSha2Result(false)
          .isKeccakResult(false)
          .isRipemdResult(false)
          .selectorKeccakResHi(false)
          .selectorSha2ResHi(false)
          .selectorRipemdResHi(false)
          .validateRow();
    }
  }

  private void traceResult(Trace.Shakiradata trace, final int stamp) {
    final boolean isShaResult = hashType == SHA256;
    final boolean isKecResult = hashType == KECCAK;
    final boolean isRipResult = hashType == RIPEMD;
    final UnsignedByte phase =
        switch (hashType) {
          case SHA256 -> UnsignedByte.of(PHASE_SHA2_RESULT);
          case KECCAK -> UnsignedByte.of(PHASE_KECCAK_RESULT);
          case RIPEMD -> UnsignedByte.of(PHASE_RIPEMD_RESULT);
        };

    for (int ct = 0; ct <= INDEX_MAX_RESULT; ct++) {
      trace
          .shakiraStamp(stamp)
          .id(ID)
          .phase(phase)
          .index(ct)
          .indexMax(INDEX_MAX_RESULT)
          .isSha2Data(false)
          .isKeccakData(false)
          .isRipemdData(false)
          .isSha2Result(isShaResult)
          .isKeccakResult(isKecResult)
          .isRipemdResult(isRipResult)
          .nBytes((short) LLARGE)
          .totalSize(WORD_SIZE);

      switch (ct) {
        case 0 ->
            trace
                .limb(result.slice(0, LLARGE))
                .nBytesAcc(LLARGE)
                .selectorKeccakResHi(hashType == KECCAK)
                .selectorSha2ResHi(hashType == SHA256)
                .selectorRipemdResHi(hashType == RIPEMD)
                .validateRow();
        case 1 ->
            trace
                .limb(result.slice(LLARGE, LLARGE))
                .nBytesAcc(WORD_SIZE)
                .selectorKeccakResHi(false)
                .selectorSha2ResHi(false)
                .selectorRipemdResHi(false)
                .validateRow();
      }
    }
  }

  @Override
  public String toString() {
    return "ShakiraDataOperation{"
        + "hashType="
        + hashType
        + ", hashInput="
        + bytesToHex(hashInput.toArray())
        + ", ID="
        + ID
        + ", inputSize="
        + inputSize
        + ", lastNBytes="
        + lastNBytes
        + ", indexMaxData="
        + indexMaxData
        + '}';
  }
}

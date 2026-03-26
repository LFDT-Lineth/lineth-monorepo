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

package net.consensys.linea.zktracer.module.blockhash;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.Trace.Blockhash.BLOCKHASH_DEPTH;
import static net.consensys.linea.zktracer.Trace.Blockhash.nROWS_PRPRC;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Hash;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class BlockhashOperation extends ModuleOperation {

  public static final short NB_ROWS_BLOCKHASH = BLOCKHASH_DEPTH;

  @Getter @EqualsAndHashCode.Include private final short relBlock;
  @Getter @EqualsAndHashCode.Include private final Bytes32 blockhashArg;
  private final long absBlock;
  @Getter private final Hash blockhashRes;
  private final Wcp wcp;

  private final Bytes[] exoArg1Hi = new Bytes[nROWS_PRPRC];
  private final Bytes[] exoArg1Lo = new Bytes[nROWS_PRPRC];
  private final Bytes[] exoArg2Hi = new Bytes[nROWS_PRPRC];
  private final Bytes[] exoArg2Lo = new Bytes[nROWS_PRPRC];
  private final long[] exoInst = new long[nROWS_PRPRC];
  private final boolean[] exoRes = new boolean[nROWS_PRPRC];

  public BlockhashOperation(
      final short relBlock,
      final long absBlock,
      final Bytes32 blockhashArg,
      final Hash blockhashRes,
      final Wcp wcp) {
    this.relBlock = relBlock;
    this.absBlock = absBlock;
    this.blockhashArg = blockhashArg;
    this.blockhashRes = blockhashRes;
    this.wcp = wcp;
  }

  void handlePreprocessing(Bytes32 prevBlockhashArg) {

    // NOTE: w goes from 0 to 4 because it refers to the array
    // however, rows go from i+1 to i+5 because it refers the MACRO row (index i)
    // row i + 1
    wcpCallToLEQ(0, prevBlockhashArg, blockhashArg);

    // row i + 2
    wcpCallToEQ(1, prevBlockhashArg, blockhashArg);

    // row i + 3
    final boolean blockNumberGreaterThan256 =
        wcpCallToLEQ(
            2,
            Bytes32.leftPad(Bytes.minimalBytes(BLOCKHASH_MAX_HISTORY)),
            Bytes32.leftPad(Bytes.ofUnsignedLong(absBlock)));
    final long minimalReachable = blockNumberGreaterThan256 ? absBlock - BLOCKHASH_MAX_HISTORY : 0;

    // row i + 4
    final boolean upperBoundOk =
        wcpCallToLT(3, blockhashArg, Bytes32.leftPad(Bytes.ofUnsignedLong(absBlock)));

    // row i + 5
    final boolean lowerBoundOk =
        wcpCallToLEQ(4, Bytes32.leftPad(Bytes.ofUnsignedLong(minimalReachable)), blockhashArg);
  }

  @Override
  protected int computeLineCount() {
    return NB_ROWS_BLOCKHASH;
  }

  public void traceMacro(Trace.Blockhash trace, final Bytes32 blockhashVal) {
    trace
        .iomf(true)
        .macro(true)
        .ct(0)
        .ctMax(0)
        .pMacroRelBlock(relBlock)
        .pMacroAbsBlock(absBlock)
        .pMacroBlockhashValHi(blockhashVal.slice(0, LLARGE))
        .pMacroBlockhashValLo(blockhashVal.slice(LLARGE, LLARGE))
        .pMacroBlockhashArgHi(blockhashArg.slice(0, LLARGE))
        .pMacroBlockhashArgLo(blockhashArg.slice(LLARGE, LLARGE))
        .pMacroBlockhashResHi(blockhashRes.getBytes().slice(0, LLARGE))
        .pMacroBlockhashResLo(blockhashRes.getBytes().slice(LLARGE, LLARGE))
        .fillAndValidateRow();
  }

  public void tracePreprocessing(Trace.Blockhash trace) {
    for (int ct = 0; ct < nROWS_PRPRC; ct++) {
      trace
          .iomf(true)
          .prprc(true)
          .ct(ct)
          .ctMax(nROWS_PRPRC - 1)
          .pPreprocessingExoInst(exoInst[ct])
          .pPreprocessingExoArg1Hi(exoArg1Hi[ct])
          .pPreprocessingExoArg1Lo(exoArg1Lo[ct])
          .pPreprocessingExoArg2Hi(exoArg2Hi[ct])
          .pPreprocessingExoArg2Lo(exoArg2Lo[ct])
          .pPreprocessingExoRes(exoRes[ct])
          .fillAndValidateRow();
    }
  }

  // WCP calls
  private boolean wcpCallToLT(int w, Bytes32 exoArg1, Bytes32 exoArg2) {
    this.exoInst[w] = EVM_INST_LT;
    this.exoArg1Hi[w] = exoArg1.slice(0, LLARGE);
    this.exoArg1Lo[w] = exoArg1.slice(LLARGE, LLARGE);
    this.exoArg2Hi[w] = exoArg2.slice(0, LLARGE);
    this.exoArg2Lo[w] = exoArg2.slice(LLARGE, LLARGE);
    this.exoRes[w] = wcp.callLT(exoArg1, exoArg2);
    return this.exoRes[w];
  }

  private boolean wcpCallToLEQ(int w, Bytes32 exoArg1, Bytes32 exoArg2) {
    this.exoInst[w] = WCP_INST_LEQ;
    this.exoArg1Hi[w] = exoArg1.slice(0, LLARGE);
    this.exoArg1Lo[w] = exoArg1.slice(LLARGE, LLARGE);
    this.exoArg2Hi[w] = exoArg2.slice(0, LLARGE);
    this.exoArg2Lo[w] = exoArg2.slice(LLARGE, LLARGE);
    this.exoRes[w] = wcp.callLEQ(exoArg1, exoArg2);
    return this.exoRes[w];
  }

  private boolean wcpCallToEQ(int w, Bytes32 exoArg1, Bytes32 exoArg2) {
    this.exoInst[w] = EVM_INST_EQ;
    this.exoArg1Hi[w] = exoArg1.slice(0, LLARGE);
    this.exoArg1Lo[w] = exoArg1.slice(LLARGE, LLARGE);
    this.exoArg2Hi[w] = exoArg2.slice(0, LLARGE);
    this.exoArg2Lo[w] = exoArg2.slice(LLARGE, LLARGE);
    this.exoRes[w] = wcp.callEQ(exoArg1, exoArg2);
    return this.exoRes[w];
  }
}

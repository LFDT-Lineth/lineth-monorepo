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

package net.consensys.linea.zktracer.module.txndata.london;

import static net.consensys.linea.zktracer.Trace.EVM_INST_ISZERO;
import static net.consensys.linea.zktracer.Trace.EVM_INST_LT;
import static net.consensys.linea.zktracer.Trace.WCP_INST_LEQ;
import static net.consensys.linea.zktracer.types.Conversions.booleanToBytes;

import lombok.Builder;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

@Builder
public record TxnDataComparisonRecord(
    boolean wcpFlag, boolean eucFlag, int instruction, Bytes arg1, Bytes arg2, Bytes result) {

  public static TxnDataComparisonRecord callToEuc(
      final Euc euc, final Bytes arg1, final Bytes arg2) {
    final Bytes result = euc.callEUC(arg1, arg2).quotient();
    return TxnDataComparisonRecord.builder()
        .wcpFlag(false)
        .eucFlag(true)
        .instruction(0)
        .arg1(arg1)
        .arg2(arg2)
        .result(result)
        .build();
  }

  public static TxnDataComparisonRecord callToLt(
      final Wcp wcp, final Bytes arg1, final Bytes arg2) {
    return TxnDataComparisonRecord.builder()
        .wcpFlag(true)
        .eucFlag(false)
        .instruction(EVM_INST_LT)
        .arg1(arg1)
        .arg2(arg2)
        .result(booleanToBytes(wcp.callLT(arg1, arg2)))
        .build();
  }

  public static TxnDataComparisonRecord callToLeq(
      final Wcp wcp, final Bytes arg1, final Bytes arg2) {
    return TxnDataComparisonRecord.builder()
        .wcpFlag(true)
        .eucFlag(false)
        .instruction(WCP_INST_LEQ)
        .arg1(arg1)
        .arg2(arg2)
        .result(booleanToBytes(wcp.callLEQ(arg1, arg2)))
        .build();
  }

  public static TxnDataComparisonRecord callToIszero(final Wcp wcp, final Bytes arg1) {
    return TxnDataComparisonRecord.builder()
        .wcpFlag(true)
        .eucFlag(false)
        .instruction(EVM_INST_ISZERO)
        .arg1(arg1)
        .arg2(Bytes.EMPTY)
        .result(booleanToBytes(wcp.callISZERO(arg1)))
        .build();
  }

  public static TxnDataComparisonRecord empty() {
    return TxnDataComparisonRecord.builder()
        .wcpFlag(false)
        .eucFlag(false)
        .instruction(0)
        .arg1(Bytes.EMPTY)
        .arg2(Bytes.EMPTY)
        .result(Bytes.EMPTY)
        .build();
  }
}

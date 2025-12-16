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

package net.consensys.linea.zktracer.module.logdata;

import static net.consensys.linea.zktracer.module.ModuleName.LOG_DATA;
import static net.consensys.linea.zktracer.types.Utils.fromDataSizeToLimbCtMax;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;

import java.util.List;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.container.stacked.CountOnlyOperation;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.rlptxrcpt.RlpTxnRcpt;
import net.consensys.linea.zktracer.module.rlptxrcpt.RlpTxrcptOperation;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.log.Log;

@RequiredArgsConstructor
public class LogData implements Module {
  private final RlpTxnRcpt rlpTxnRcpt;
  private final CountOnlyOperation lineCounter = new CountOnlyOperation();

  @Override
  public ModuleName moduleKey() {
    return LOG_DATA;
  }

  @Override
  public void commitTransactionBundle() {
    lineCounter.commitTransactionBundle();
  }

  @Override
  public void popTransactionBundle() {
    lineCounter.popTransactionBundle();
  }

  /* WARN: make sure this is called after rlpTxnRcpt as we need the operation of the current transaction */
  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    lineCounter.add(lineCountForLogData(rlpTxnRcpt.operations().getLast()));
  }

  @Override
  public int lineCount() {
    return lineCounter.lineCount();
  }

  @Override
  public int spillage(Trace trace) {
    return trace.logdata().spillage();
  }

  private int lineCountForLogData(RlpTxrcptOperation tx) {
    return lineCountForLogData(tx.logs());
  }

  public static int lineCountForLogData(List<Log> logs) {
    int txRowSize = 0;
    if (logs.isEmpty()) {
      return 0;
    } else {
      for (Log log : logs) {
        txRowSize += indexMax(log) + 1;
      }
      return txRowSize;
    }
  }

  private static int indexMax(Log log) {
    return log.getData().isEmpty() ? 0 : fromDataSizeToLimbCtMax(log.getData().size());
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.logdata().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    int absLogNumMax = 0;
    for (RlpTxrcptOperation tx : rlpTxnRcpt.operations().getAll()) {
      absLogNumMax += tx.logs().size();
    }

    int absLogNum = 0;
    for (RlpTxrcptOperation tx : rlpTxnRcpt.operations().getAll()) {
      if (!tx.logs().isEmpty()) {
        for (Log log : tx.logs()) {
          absLogNum += 1;
          if (log.getData().isEmpty()) {
            traceLogWoData(absLogNum, absLogNumMax, trace.logdata());
          } else {
            traceLog(log, absLogNum, absLogNumMax, trace.logdata());
          }
        }
      }
    }
  }

  public void traceLogWoData(final int absLogNum, final int absLogNumMax, Trace.Logdata trace) {
    trace
        .absLogNumMax(absLogNumMax)
        .absLogNum(absLogNum)
        .logsData(false)
        .sizeTotal(0)
        .sizeAcc(0)
        .sizeLimb(0)
        .limb(Bytes.EMPTY)
        .index(0)
        .validateRow();
  }

  public void traceLog(
      final Log log, final int absLogNum, final int absLogNumMax, Trace.Logdata trace) {
    final int indexMax = indexMax(log);
    final Bytes dataPadded = rightPadTo(log.getData(), (indexMax + 1) * 16);
    final int lastLimbSize = (log.getData().size() % 16 == 0) ? 16 : log.getData().size() % 16;
    for (int index = 0; index < indexMax + 1; index++) {
      trace
          .absLogNumMax(absLogNumMax)
          .absLogNum(absLogNum)
          .logsData(true)
          .sizeTotal(log.getData().size())
          .sizeAcc(index == indexMax ? log.getData().size() : 16L * (index + 1))
          .sizeLimb(index == indexMax ? lastLimbSize : 16)
          .limb(dataPadded.slice(16 * index, 16))
          .index(index)
          .validateRow();
    }
  }
}

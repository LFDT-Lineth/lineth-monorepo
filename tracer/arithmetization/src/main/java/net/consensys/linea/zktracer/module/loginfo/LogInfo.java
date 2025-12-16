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

package net.consensys.linea.zktracer.module.loginfo;

import static net.consensys.linea.zktracer.Trace.EVM_INST_LOG0;
import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_ADDR;
import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_DATA_SIZE;
import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_NO_LOG_ENTRY;
import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_TOPIC_BASE;
import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA;
import static net.consensys.linea.zktracer.module.ModuleName.LOG_INFO;

import java.util.List;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.container.stacked.CountOnlyOperation;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.rlptxrcpt.RlpTxnRcpt;
import net.consensys.linea.zktracer.module.rlptxrcpt.RlpTxrcptOperation;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.evm.log.Log;

@RequiredArgsConstructor
public class LogInfo implements Module {
  private final RlpTxnRcpt rlpTxnRcpt;
  private final CountOnlyOperation lineCounter = new CountOnlyOperation();

  @Override
  public ModuleName moduleKey() {
    return LOG_INFO;
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
    lineCounter.add(lineCountForLogInfo(rlpTxnRcpt.operations().getLast()));
  }

  @Override
  public int lineCount() {
    return lineCounter.lineCount();
  }

  @Override
  public int spillage(Trace trace) {
    return trace.loginfo().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.loginfo().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    int absLogNumMax = 0;
    for (RlpTxrcptOperation tx : rlpTxnRcpt.operations().getAll()) {
      absLogNumMax += tx.logs().size();
    }

    int absTxNum = 0;
    int absLogNum = 0;
    for (RlpTxrcptOperation tx : rlpTxnRcpt.operations().getAll()) {
      absTxNum += 1;
      if (tx.logs().isEmpty()) {
        traceTxWoLog(absTxNum, absLogNum, absLogNumMax, trace.loginfo());
      } else {
        for (Log log : tx.logs()) {
          absLogNum += 1;
          traceLog(log, absTxNum, absLogNum, absLogNumMax, trace.loginfo());
        }
      }
    }
  }

  private int lineCountForLogInfo(RlpTxrcptOperation tx) {
    return lineCountForLogInfo(tx.logs());
  }

  public static int lineCountForLogInfo(List<Log> logs) {
    int txRowSize = 0;
    if (logs.isEmpty()) {
      return 1;
    } else {
      for (Log log : logs) {
        txRowSize += ctMax(log) + 1;
      }
      return txRowSize;
    }
  }

  public void traceTxWoLog(
      final int absTxNum, final int absLogNum, final int absLogNumMax, Trace.Loginfo trace) {
    trace
        .absTxnNumMax(rlpTxnRcpt.operations().size())
        .absTxnNum(absTxNum)
        .txnEmitsLogs(false)
        .absLogNumMax(absLogNumMax)
        .absLogNum(absLogNum)
        .ctMax(UnsignedByte.ZERO)
        .ct(UnsignedByte.ZERO)
        .addrHi(0L)
        .addrLo(Bytes.EMPTY)
        .topicHi1(Bytes.EMPTY)
        .topicLo1(Bytes.EMPTY)
        .topicHi2(Bytes.EMPTY)
        .topicLo2(Bytes.EMPTY)
        .topicHi3(Bytes.EMPTY)
        .topicLo3(Bytes.EMPTY)
        .topicHi4(Bytes.EMPTY)
        .topicLo4(Bytes.EMPTY)
        .dataSize(0L)
        .inst(UnsignedByte.ZERO)
        .isLogX0(false)
        .isLogX1(false)
        .isLogX2(false)
        .isLogX3(false)
        .isLogX4(false)
        .phase(RLP_RCPT_SUBPHASE_ID_NO_LOG_ENTRY)
        .dataHi(Bytes.EMPTY)
        .dataLo(Bytes.EMPTY)
        .validateRow();
  }

  public void traceLog(
      final Log log,
      final int absTxNum,
      final int absLogNum,
      final int absLogNumMax,
      Trace.Loginfo trace) {
    final int ctMax = ctMax(log);
    final int nbTopic = log.getTopics().size();
    final Bytes32 topic1 = nbTopic >= 1 ? log.getTopics().get(0) : Bytes32.ZERO;
    final Bytes32 topic2 = nbTopic >= 2 ? log.getTopics().get(1) : Bytes32.ZERO;
    final Bytes32 topic3 = nbTopic >= 3 ? log.getTopics().get(2) : Bytes32.ZERO;
    final Bytes32 topic4 = nbTopic >= 4 ? log.getTopics().get(3) : Bytes32.ZERO;
    for (int ct = 0; ct < ctMax + 1; ct++) {
      trace
          .absTxnNumMax(this.rlpTxnRcpt.operations().size())
          .absTxnNum(absTxNum)
          .txnEmitsLogs(true)
          .absLogNumMax(absLogNumMax)
          .absLogNum(absLogNum)
          .ctMax(UnsignedByte.of(ctMax))
          .ct(UnsignedByte.of(ct))
          .addrHi(log.getLogger().slice(0, 4).toLong())
          .addrLo(log.getLogger().slice(4, 16))
          .topicHi1(topic1.slice(0, 16))
          .topicLo1(topic1.slice(16, 16))
          .topicHi2(topic2.slice(0, 16))
          .topicLo2(topic2.slice(16, 16))
          .topicHi3(topic3.slice(0, 16))
          .topicLo3(topic3.slice(16, 16))
          .topicHi4(topic4.slice(0, 16))
          .topicLo4(topic4.slice(16, 16))
          .dataSize(log.getData().size())
          .inst(UnsignedByte.of(EVM_INST_LOG0 + nbTopic))
          .isLogX0(nbTopic == 0)
          .isLogX1(nbTopic == 1)
          .isLogX2(nbTopic == 2)
          .isLogX3(nbTopic == 3)
          .isLogX4(nbTopic == 4);

      switch (ct) {
        case 0 -> {
          trace
              .phase(RLP_RCPT_SUBPHASE_ID_DATA_SIZE)
              .dataHi(Bytes.ofUnsignedInt(log.getData().size()))
              .dataLo(Bytes.ofUnsignedInt(nbTopic))
              .validateRow();
        }
        case 1 -> {
          trace
              .phase(RLP_RCPT_SUBPHASE_ID_ADDR)
              .dataHi(log.getLogger().slice(0, 4))
              .dataLo(log.getLogger().slice(4, 16))
              .validateRow();
        }
        case 2 -> {
          trace
              .phase(RLP_RCPT_SUBPHASE_ID_TOPIC_BASE + RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA)
              .dataHi(topic1.slice(0, 16))
              .dataLo(topic1.slice(16, 16))
              .validateRow();
        }
        case 3 -> {
          trace
              .phase(RLP_RCPT_SUBPHASE_ID_TOPIC_BASE + 2 * RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA)
              .dataHi(topic2.slice(0, 16))
              .dataLo(topic2.slice(16, 16))
              .validateRow();
        }
        case 4 -> {
          trace
              .phase(RLP_RCPT_SUBPHASE_ID_TOPIC_BASE + 3 * RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA)
              .dataHi(topic3.slice(0, 16))
              .dataLo(topic3.slice(16, 16))
              .validateRow();
        }
        case 5 -> {
          trace
              .phase(RLP_RCPT_SUBPHASE_ID_TOPIC_BASE + 4 * RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA)
              .dataHi(topic4.slice(0, 16))
              .dataLo(topic4.slice(16, 16))
              .validateRow();
        }
        default ->
            throw new IllegalArgumentException("ct = " + ct + " greater than ctMax =" + ctMax);
      }
    }
  }

  private static int ctMax(Log log) {
    return log.getTopics().size() + 1;
  }
}

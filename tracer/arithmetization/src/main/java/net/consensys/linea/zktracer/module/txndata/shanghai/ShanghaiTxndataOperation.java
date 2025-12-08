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

package net.consensys.linea.zktracer.module.txndata.shanghai;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.TraceShanghai.Txndata.*;

import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.txndata.london.LondonTxnDataOperation;
import net.consensys.linea.zktracer.module.txndata.london.TxnDataComparisonRecord;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;

public class ShanghaiTxndataOperation extends LondonTxnDataOperation {

  public static final Bytes MAX_INIT_CODE_SIZE_BYTES = Bytes.ofUnsignedInt(MAX_INIT_CODE_SIZE);
  private static final Bytes WORD_SIZE_BYTES = Bytes.ofUnsignedInt(WORD_SIZE);

  public ShanghaiTxndataOperation(
      Wcp wcp,
      Euc euc,
      TransactionProcessingMetadata tx,
      int nbRowsType0,
      int nbRowsType1,
      int nbRowsType2,
      int nbWcpEucRowsFrontierAccessList) {
    super(wcp, euc, tx, nbRowsType0, nbRowsType1, nbRowsType2, nbWcpEucRowsFrontierAccessList);
  }

  @Override
  protected int computeLineCount() {
    // Count the number of rows of each tx, only depending on the type of the transaction
    return switch (tx.getBesuTransaction().getType()) {
      case FRONTIER -> NB_ROWS_TYPE_0;
      case ACCESS_LIST -> NB_ROWS_TYPE_1;
      case EIP1559 -> NB_ROWS_TYPE_2;
      default -> throw new RuntimeException(
          "Transaction type not supported:" + tx.getBesuTransaction().getType());
    };
  }

  @Override
  public void setShanghaiCallsToEucAndWcp() {
    if (tx.isDeployment()) {
      // row 2
      callsToEucAndWcp.add(
          TxnDataComparisonRecord.callToLeq(
              wcp,
              Bytes.ofUnsignedInt(tx.getBesuTransaction().getPayload().size()),
              MAX_INIT_CODE_SIZE_BYTES));
      // row 3
      final Bytes divisor =
          Bytes.minimalBytes(tx.getBesuTransaction().getPayload().size() + WORD_SIZE_MO);
      callsToEucAndWcp.add(TxnDataComparisonRecord.callToEuc(euc, divisor, WORD_SIZE_BYTES));
    } else {
      // rows 2 & 3
      callsToEucAndWcp.add(TxnDataComparisonRecord.empty());
      callsToEucAndWcp.add(TxnDataComparisonRecord.empty());
    }
  }
}

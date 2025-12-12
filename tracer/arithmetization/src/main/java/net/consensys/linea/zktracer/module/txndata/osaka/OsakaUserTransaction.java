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

package net.consensys.linea.zktracer.module.txndata.osaka;

import static net.consensys.linea.zktracer.TraceOsaka.EIP_7825_TRANSACTION_GAS_LIMIT_CAP;

import net.consensys.linea.zktracer.module.txndata.cancun.CancunTxnData;
import net.consensys.linea.zktracer.module.txndata.cancun.rows.computationRows.WcpRow;
import net.consensys.linea.zktracer.module.txndata.cancun.transactions.CancunUserTransaction;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;

public class OsakaUserTransaction extends CancunUserTransaction {
  public static final short NB_ROWS_TXN_DATA_OSAKA_USER_1559_SEMANTIC = 17;
  public static final short NB_ROWS_TXN_DATA_OSAKA_USER_NO_1559_SEMANTIC = 15;
  private static final Bytes EIP_7825_TRANSACTION_GAS_LIMIT_CAP_BYTES =
      Bytes.minimalBytes(EIP_7825_TRANSACTION_GAS_LIMIT_CAP);

  public OsakaUserTransaction(CancunTxnData txnData, TransactionProcessingMetadata txnMetadata) {
    super(txnData, txnMetadata);
  }

  @Override
  protected void eip7825TransactionGasLimitCap() {
    final long gasLimit = txn.getGasLimit();

    final WcpRow transactionGasLimitCap =
        WcpRow.smallCallToLeq(
            wcp, Bytes.ofUnsignedLong(gasLimit), EIP_7825_TRANSACTION_GAS_LIMIT_CAP_BYTES);

    rows.add(transactionGasLimitCap);
  }

  @Override
  protected int ctMax() {
    return (txn.transactionTypeHasEip1559GasSemantics()
            ? NB_ROWS_TXN_DATA_OSAKA_USER_1559_SEMANTIC
            : NB_ROWS_TXN_DATA_OSAKA_USER_NO_1559_SEMANTIC)
        - 1;
  }
}

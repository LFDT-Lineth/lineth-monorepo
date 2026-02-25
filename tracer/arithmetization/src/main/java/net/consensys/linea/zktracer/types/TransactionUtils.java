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

package net.consensys.linea.zktracer.types;

import org.hyperledger.besu.datatypes.Transaction;

public class TransactionUtils {
  public static long getChainIdFromTransaction(final Transaction tx) {
    switch (tx.getType()) {
      case ACCESS_LIST, EIP1559, BLOB -> {
        return tx.getChainId().get().longValueExact();
      }
      case FRONTIER -> {
        // If chainId is specified, V = 2 * ChainID + 35 or V = 2 * ChainId + 36;
        final long V = tx.getV().longValueExact();
        if (V == 27 || V == 28) {
          throw new IllegalArgumentException("ChainId not specified in transaction");
        }
        return (V - 35) / 2;
      }
      default ->
          throw new IllegalArgumentException("Transaction type not supported: " + tx.getType());
    }
  }

  public static boolean transactionHasEip1559GasSemantics(Transaction tx) {
    return tx.getType().supports1559FeeMarket();
  }

  public static boolean transactionSupportsDelegateCode(Transaction tx) {
    return tx.getType().supportsDelegateCode();
  }
}

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

package net.consensys.linea.zktracer.module.rlptxn.london;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.module.Util.getTxTypeAsInt;

import java.math.BigInteger;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.rlptxn.RlpTxnOperation;
import org.hyperledger.besu.datatypes.Transaction;

@Accessors(fluent = true)
@Getter
public final class LondonRlpTxnOperation extends RlpTxnOperation {
  private final Transaction tx;
  private final boolean requireEvmExecution;

  public LondonRlpTxnOperation(Transaction tx, boolean requiresEvmExecution) {
    this.tx = tx;
    this.requireEvmExecution = requiresEvmExecution;
  }

  @Override
  protected int computeLineCount() {
    final int txType = getTxTypeAsInt(this.tx.getType());
    // Phase RLP prefix is always 17 rows long
    int rowSize = 17;

    // Phase chainID
    if (txType == 1 || txType == 2) {
      if (this.tx.getChainId().orElseThrow().equals(BigInteger.ZERO)) {
        rowSize += 1;
      } else {
        rowSize += 8;
      }
    }

    // Phase nonce
    if (this.tx.getNonce() == 0) {
      rowSize += 1;
    } else {
      rowSize += 8;
    }

    // Phase gasPrice
    if (txType == 0 || txType == 1) {
      rowSize += 8;
    }

    // Phase MaxPriorityFeeperGas
    if (txType == 2) {
      if (this.tx
          .getMaxPriorityFeePerGas()
          .orElseThrow()
          .getAsBigInteger()
          .equals(BigInteger.ZERO)) {
        rowSize += 1;
      } else {
        rowSize += 8;
      }
    }

    // Phase MaxFeePerGas
    if (txType == 2) {
      if (this.tx.getMaxFeePerGas().orElseThrow().getAsBigInteger().equals(BigInteger.ZERO)) {
        rowSize += 1;
      } else {
        rowSize += 8;
      }
    }

    // Phase GasLimit
    rowSize += 8;

    // Phase To
    if (this.tx.getTo().isPresent()) {
      rowSize += 16;
    } else {
      rowSize += 1;
    }

    // Phase Value
    if (this.tx.getValue().getAsBigInteger().equals(BigInteger.ZERO)) {
      rowSize += 1;
    } else {
      rowSize += 16;
    }

    // Phase Data
    if (this.tx.getPayload().isEmpty()) {
      rowSize += 2; // 1 for prefix + 1 for padding
    } else {
      final int dataSize = this.tx.getPayload().size();
      rowSize += 8 + LLARGE * ((dataSize - 1) / LLARGE + 1);
      rowSize += 2; // 2 lines of padding
    }

    // Phase AccessList
    if (txType == 1 || txType == 2) {
      if (this.tx.getAccessList().isEmpty() || this.tx.getAccessList().get().isEmpty()) {
        rowSize += 1;
      } else {
        // Rlp prefix of the AccessList list
        rowSize += 8;
        for (int i = 0; i < this.tx.getAccessList().orElseThrow().size(); i++) {
          rowSize += 8 + 16;
          if (this.tx.getAccessList().orElseThrow().get(i).storageKeys().isEmpty()) {
            rowSize += 1;
          } else {
            rowSize += 8 + 16 * this.tx.getAccessList().orElseThrow().get(i).storageKeys().size();
          }
        }
      }
    }

    // Phase beta
    if (txType == 0) {
      rowSize += 8;
      if (this.tx.getV().compareTo(BigInteger.valueOf(28)) > 0) {
        rowSize += 9;
      }
    }

    // Phase y
    if (txType == 1 || txType == 2) {
      rowSize += 1;
    }

    // Phase r
    if (this.tx.getR().equals(BigInteger.ZERO)) {
      rowSize += 1;
    } else {
      rowSize += 16;
    }

    // Phase s
    if (this.tx.getS().equals(BigInteger.ZERO)) {
      rowSize += 1;
    } else {
      rowSize += 16;
    }
    return rowSize;
  }
}

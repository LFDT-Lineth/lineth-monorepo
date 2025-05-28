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
package net.consensys.linea.sequencer.txselection;

import org.hyperledger.besu.plugin.data.TransactionSelectionResult;

public class LineaTransactionSelectionResult extends TransactionSelectionResult {
  private enum LineaStatus implements TransactionSelectionResult.Status {
    BLOCK_CALLDATA_OVERFLOW(false, false, false),
    BLOCK_MODULE_LINE_COUNT_FULL(true, false, false),
    TX_GAS_EXCEEDS_USER_MAX_BLOCK_GAS(false, true, true),
    TX_TOO_LARGE_FOR_REMAINING_USER_GAS(false, false, false),
    TX_MODULE_LINE_COUNT_OVERFLOW(false, true, true),
    TX_MODULE_LINE_COUNT_OVERFLOW_CACHED(false, true, true),
    TX_MODULE_LINE_INVALID_COUNT(false, true, true),
    TX_UNPROFITABLE(false, false, true),
    TX_UNPROFITABLE_UPFRONT(false, false, true),
    TX_UNPROFITABLE_RETRY_LIMIT(false, false, false),
    BUNDLE_GAS_EXCEEDS_MAX_BUNDLE_BLOCK_GAS(false, true, true),
    BUNDLE_TOO_LARGE_FOR_REMAINING_BUNDLE_BLOCK_GAS(false, false, false);

    private final boolean stop;
    private final boolean discard;
    private final boolean penalize;

    LineaStatus(boolean stop, boolean discard, boolean penalize) {
      this.stop = stop;
      this.discard = discard;
      this.penalize = penalize;
    }

    @Override
    public boolean stop() {
      return stop;
    }

    @Override
    public boolean discard() {
      return discard;
    }

    @Override
    public boolean penalize() {
      return penalize;
    }
  }

  protected LineaTransactionSelectionResult(LineaStatus status) {
    super(status);
  }

  public static final TransactionSelectionResult BLOCK_CALLDATA_OVERFLOW =
      new LineaTransactionSelectionResult(LineaStatus.BLOCK_CALLDATA_OVERFLOW);
  public static final TransactionSelectionResult BLOCK_MODULE_LINE_COUNT_FULL =
      new LineaTransactionSelectionResult(LineaStatus.BLOCK_MODULE_LINE_COUNT_FULL);
  public static final TransactionSelectionResult TX_GAS_EXCEEDS_USER_MAX_BLOCK_GAS =
      new LineaTransactionSelectionResult(LineaStatus.TX_GAS_EXCEEDS_USER_MAX_BLOCK_GAS);
  public static final TransactionSelectionResult TX_TOO_LARGE_FOR_REMAINING_USER_GAS =
      new LineaTransactionSelectionResult(LineaStatus.TX_TOO_LARGE_FOR_REMAINING_USER_GAS);
  public static final TransactionSelectionResult TX_MODULE_LINE_COUNT_OVERFLOW =
      new LineaTransactionSelectionResult(LineaStatus.TX_MODULE_LINE_COUNT_OVERFLOW);
  public static final TransactionSelectionResult TX_MODULE_LINE_COUNT_OVERFLOW_CACHED =
      new LineaTransactionSelectionResult(LineaStatus.TX_MODULE_LINE_COUNT_OVERFLOW_CACHED);
  public static final TransactionSelectionResult TX_MODULE_LINE_INVALID_COUNT =
      new LineaTransactionSelectionResult(LineaStatus.TX_MODULE_LINE_INVALID_COUNT);
  public static final TransactionSelectionResult TX_UNPROFITABLE =
      new LineaTransactionSelectionResult(LineaStatus.TX_UNPROFITABLE);
  public static final TransactionSelectionResult TX_UNPROFITABLE_UPFRONT =
      new LineaTransactionSelectionResult(LineaStatus.TX_UNPROFITABLE_UPFRONT);
  public static final TransactionSelectionResult TX_UNPROFITABLE_RETRY_LIMIT =
      new LineaTransactionSelectionResult(LineaStatus.TX_UNPROFITABLE_RETRY_LIMIT);
  public static final TransactionSelectionResult BUNDLE_GAS_EXCEEDS_MAX_BUNDLE_BLOCK_GAS =
      new LineaTransactionSelectionResult(LineaStatus.BUNDLE_GAS_EXCEEDS_MAX_BUNDLE_BLOCK_GAS);
  public static final TransactionSelectionResult BUNDLE_TOO_LARGE_FOR_REMAINING_BUNDLE_BLOCK_GAS =
      new LineaTransactionSelectionResult(
          LineaStatus.BUNDLE_TOO_LARGE_FOR_REMAINING_BUNDLE_BLOCK_GAS);
}

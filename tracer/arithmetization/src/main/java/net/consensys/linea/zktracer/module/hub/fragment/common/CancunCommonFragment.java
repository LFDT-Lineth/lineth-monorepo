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

package net.consensys.linea.zktracer.module.hub.fragment.common;

import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.*;

import net.consensys.linea.zktracer.Trace;

public class CancunCommonFragment extends LondonCommonFragment {
  public CancunCommonFragment(
      CommonFragmentValues commonValues,
      int stackLineCounter,
      int nonStackLineCounter,
      int mmuStamp,
      int mxpStamp) {
    super(commonValues, stackLineCounter, nonStackLineCounter, mmuStamp, mxpStamp);
  }

  @Override
  protected void traceTransactionsAndBlockNumbers(Trace.Hub trace) {
    trace
        .sysiTxnNumber(commonFragmentValues.sysiTransactionNumber)
        .userTxnNumber(commonFragmentValues.userTransactionNumber)
        .sysfTxnNumber(commonFragmentValues.sysfTransactionNumber)
        .blkNumber(commonFragmentValues.relBlockNumber);
  }

  @Override
  protected void traceTransactionProcessingType(Trace.Hub trace) {
    trace
        .sysi(commonFragmentValues.transactionProcessingType == SYSI)
        .user(commonFragmentValues.transactionProcessingType == USER)
        .sysf(commonFragmentValues.transactionProcessingType == SYSF);
  }
}

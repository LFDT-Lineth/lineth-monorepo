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

import net.consensys.linea.zktracer.Trace;

public class LondonCommonFragment extends CommonFragment {

  public LondonCommonFragment(
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
        .absoluteTransactionNumber(tx().getUserTransactionNumber())
        .relativeBlockNumber(tx().getRelativeBlockNumber());
  }

  @Override
  protected void traceTransactionProcessingType(Trace.Hub trace) {
    // The transaction processing type is not used in London, so we do not trace it.
  }
}

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

package net.consensys.linea.zktracer.module.rlptxn.cancun;

import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlptxn.RlpTxn;
import net.consensys.linea.zktracer.module.rlptxn.RlpTxnOperation;
import net.consensys.linea.zktracer.module.trm.Trm;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;

@RequiredArgsConstructor
public class CancunRlpTxn extends RlpTxn {
  private final RlpUtils rlpUtils;
  private final Trm trm;

  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    operations.add(new CancunRlpTxnOperation(rlpUtils, trm, tx));
  }

  @Override
  protected void traceOperation(RlpTxnOperation op, int userTxNumber, Trace.Rlptxn trace) {
    final CancunRlpTxnOperation cancunOp = (CancunRlpTxnOperation) op;
    cancunOp.trace(trace, operations.size());
  }
}

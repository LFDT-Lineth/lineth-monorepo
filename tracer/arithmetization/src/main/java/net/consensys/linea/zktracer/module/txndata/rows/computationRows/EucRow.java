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
package net.consensys.linea.zktracer.module.txndata.rows.computationRows;

import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.euc.EucOperation;
import org.apache.tuweni.bytes.Bytes;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class EucRow extends ComputationRow {

  final long dividend;
  final long divisor;
  @Getter final long quotient;
  @Getter final long remainder;

  public static EucRow callToEuc(Euc euc, final long dividend, final long divisor) {
    EucOperation op = euc.callEUC(Bytes.ofUnsignedLong(dividend), Bytes.ofUnsignedLong(divisor));
    return new EucRow(dividend, divisor, op.quotient().toLong(), op.remainder().toLong());
  }

  @Override
  public void traceRow(Trace.Txndata trace) {
    super.traceRow(trace);
    trace
        .pComputationEucFlag(true)
        .pComputationArg1Lo(Bytes.ofUnsignedLong(dividend))
        .pComputationArg2Lo(Bytes.ofUnsignedLong(divisor))
        // no computation/INST tracing for EUC
        .pComputationEucQuotient(Bytes.ofUnsignedLong(quotient))
        .pComputationEucRemainder(Bytes.ofUnsignedLong(remainder));
  }
}

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

package net.consensys.linea.zktracer.module.euc;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import org.apache.tuweni.bytes.Bytes;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class EucOperation extends ModuleOperation {
  @Getter @EqualsAndHashCode.Include private final Bytes dividend;
  @Getter @EqualsAndHashCode.Include private final Bytes divisor;
  @Getter private final Bytes remainder;
  @Getter private final Bytes quotient;

  public EucOperation(
      final Bytes dividend, final Bytes divisor, final Bytes quotient, final Bytes remainder) {
    if (divisor.isZero()) {
      throw new IllegalArgumentException("EUC module doesn't accept 0 for divisor");
    }

    this.dividend = dividend.trimLeadingZeros();
    this.divisor = divisor.trimLeadingZeros();
    this.quotient = quotient.trimLeadingZeros();
    this.remainder = remainder;
  }

  public Bytes ceiling() {
    return !remainder.isZero() && !dividend.isZero()
        ? Bytes.minimalBytes(quotient.toLong() + 1)
        : quotient;
  }

  void trace(Trace.Euc trace) {
    final Bytes ceil = this.ceiling();

    trace
        .dividend(dividend)
        .divisor(divisor)
        .quotient(quotient)
        .remainder(remainder)
        .ceil(ceil)
        .validateRow();
  }

  @Override
  protected int computeLineCount() {
    return 1;
  }
}

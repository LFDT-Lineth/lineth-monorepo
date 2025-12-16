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

package net.consensys.linea.zktracer.module.limits.precompiles;

import static java.lang.Integer.MAX_VALUE;
import static net.consensys.linea.zktracer.module.ModuleName.PRECOMPILE_BLAKE_ROUNDS;
import static net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpDataOperation.BLAKE2f_R_SIZE;

import com.google.common.base.Preconditions;
import java.math.BigInteger;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.module.CountingOnlyModule;
import org.apache.tuweni.bytes.Bytes;

@Getter
@Accessors(fluent = true)
public final class BlakeRounds extends CountingOnlyModule {
  @Setter private boolean transactionBundleContainsIllegalOperation = false;

  private static final BigInteger INTEGER_MAX_VALUE_BI = BigInteger.valueOf(MAX_VALUE);

  public BlakeRounds() {
    super(PRECOMPILE_BLAKE_ROUNDS);
  }

  @Override
  public void updateTally(final int count) {
    throw new UnsupportedOperationException("Not implemented");
  }

  public void addPrecompileLimit(final Bytes r) {
    Preconditions.checkArgument(r.size() == BLAKE2f_R_SIZE, "r is 4 bytes long");
    final BigInteger rBI = r.toUnsignedBigInteger();
    // check if r is greater or equal to Integer.MAX_VALUE
    if (rBI.compareTo(INTEGER_MAX_VALUE_BI) >= 0) {
      transactionBundleContainsIllegalOperation(true);
      return;
    }

    // check if the new lineCount would be greater or equal than Integer.MAX_VALUE
    final BigInteger totalRoundsCount = BigInteger.valueOf(counts.lineCount());
    if (rBI.add(totalRoundsCount).compareTo(INTEGER_MAX_VALUE_BI) >= 0) {
      transactionBundleContainsIllegalOperation(true);
      return;
    }

    // Then, as no overflow, add the count
    counts.add(rBI.intValueExact());
  }

  @Override
  public int lineCount() {
    return transactionBundleContainsIllegalOperation ? MAX_VALUE : super.lineCount();
  }

  @Override
  public void popTransactionBundle() {
    super.popTransactionBundle();
    transactionBundleContainsIllegalOperation(false);
  }
}

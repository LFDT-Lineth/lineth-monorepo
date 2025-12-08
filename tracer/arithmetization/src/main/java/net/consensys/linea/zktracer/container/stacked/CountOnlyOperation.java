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

package net.consensys.linea.zktracer.container.stacked;

import com.google.common.base.Preconditions;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;

@Accessors(fluent = true)
@RequiredArgsConstructor
public class CountOnlyOperation {

  private int countCommitedToTheConflation = 0;
  @Getter private int countInTransactionBundle = 0;

  public void commitTransactionBundle() {
    countCommitedToTheConflation += countInTransactionBundle;
    countInTransactionBundle = 0;
  }

  public void popTransactionBundle() {
    countInTransactionBundle = 0;
  }

  public void add(final int operationCount) {
    Preconditions.checkArgument(
        operationCount >= 0, "operationCount %s should be nonnegative", operationCount);
    countInTransactionBundle += operationCount;
  }

  public int lineCount() {
    return countCommitedToTheConflation + countInTransactionBundle;
  }

  public void clear() {
    countCommitedToTheConflation = 0;
    countInTransactionBundle = 0;
  }
}

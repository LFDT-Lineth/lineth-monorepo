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

package net.consensys.linea.zktracer.container.module;

import com.google.common.base.Preconditions;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.stacked.CountOnlyOperation;
import net.consensys.linea.zktracer.module.ModuleName;

/** A {@link CountingOnlyModule} is a {@link Module} that only counts certain outcomes. */
public class CountingOnlyModule implements Module {
  private final ModuleName moduleKey;
  private final short spillage;

  protected final CountOnlyOperation counts = new CountOnlyOperation();

  public CountingOnlyModule(ModuleName moduleKey, int spillage) {
    this.moduleKey = moduleKey;
    this.spillage = (short) spillage;
  }

  public CountingOnlyModule(ModuleName moduleKey) {
    this.moduleKey = moduleKey;
    this.spillage = 0;
  }

  @Override
  public void commitTransactionBundle() {
    counts.commitTransactionBundle();
  }

  @Override
  public ModuleName moduleKey() {
    return moduleKey;
  }

  @Override
  public void popTransactionBundle() {
    counts.popTransactionBundle();
  }

  @Override
  public int lineCount() {
    return counts.lineCount();
  }

  @Override
  public int spillage(Trace trace) {
    return spillage;
  }

  public void updateTally(final int count) {
    Preconditions.checkArgument(
        count >= 0, "CountingOnlyModule: count %s in updateTally must be nonnegative", count);
    counts.add(count);
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    throw new IllegalStateException("Module " + moduleKey + "  should never be traced");
  }

  @Override
  public void commit(Trace trace) {
    throw new IllegalStateException("Module " + moduleKey + "  should never be traced");
  }
}

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

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.container.stacked.CountOnlyOperation;

public interface OperationSetWithAdditionalRowsModule<E extends ModuleOperation>
    extends OperationSetModule<E> {

  CountOnlyOperation additionalRows();

  default void updateTally(int count) {
    additionalRows().add(count);
  }

  @Override
  default void commitTransactionBundle() {
    operations().commitTransactionBundle();
    additionalRows().commitTransactionBundle();
  }

  @Override
  default void popTransactionBundle() {
    operations().popTransactionBundle();
    additionalRows().popTransactionBundle();
  }

  @Override
  default int lineCount() {
    return operations().lineCount() + additionalRows().lineCount();
  }

  @Override
  default void commit(Trace trace) {
    assert additionalRows().lineCount() == 0
        : "Additional rows should be 0 when committing module " + this.moduleKey();
  }
}

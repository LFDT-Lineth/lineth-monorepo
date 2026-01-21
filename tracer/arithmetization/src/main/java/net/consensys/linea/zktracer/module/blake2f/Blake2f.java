package net.consensys.linea.zktracer.module.blake2f;

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

import static net.consensys.linea.zktracer.module.ModuleName.BLAKE2F;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class Blake2f implements OperationSetModule<Blake2fOperation> {

  @Getter
  private final ModuleOperationStackedSet<Blake2fOperation> operations =
      new ModuleOperationStackedSet<>();

  @Override
  public ModuleName moduleKey() {
    return BLAKE2F;
  }

  @Override
  public int lineCount() {
    return operations.lineCount();
  }

  @Override
  public int spillage(Trace trace) {
    return trace.blake2f().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.blake2f().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    for (Blake2fOperation op : operations.getAll()) {
      op.trace(trace.blake2f());
    }
  }

  public void call(Blake2fOperation blake2fOperation) {
    operations.add(blake2fOperation);
  }
}

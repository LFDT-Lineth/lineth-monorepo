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

package net.consensys.linea.zktracer.container.stacked;

import java.util.Collection;
import java.util.List;
import java.util.stream.Stream;

import com.google.common.base.Preconditions;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.ModuleOperation;
import org.jetbrains.annotations.NotNull;

/**
 * Implements a system of pseudo-stacked squashed List where {@link
 * ModuleOperationStackedList#operationsCommitedToTheConflation} represents the List of all
 * operations since the beginning of the conflation and {@link
 * ModuleOperationStackedList#operationsInTransactionBundle} represents the operations added by the
 * last bundle of transaction, that can be popped. The line counting is done by a separate {@link
 * CountOnlyOperation}.
 *
 * @param <E> the type of elements stored in the set
 */
@Accessors(fluent = true)
public class ModuleOperationStackedList<E extends ModuleOperation> extends StackedList<E> {
  private final CountOnlyOperation lineCounter = new CountOnlyOperation();
  private boolean conflationFinished = false;

  public ModuleOperationStackedList() {
    super();
  }

  /** Prefer this constructor as we preallocate more needed memory */
  public ModuleOperationStackedList(
      final int expectedConflationNumberOperations, final int expectedTransactionNumberOperations) {
    super(expectedConflationNumberOperations, expectedTransactionNumberOperations);
  }

  public void commitTransactionBundle() {
    super.commitTransactionBundle();
    lineCounter.commitTransactionBundle();
  }

  public void popTransactionBundle() {
    super.popTransactionBundle();
    lineCounter.popTransactionBundle();
  }

  public int lineCount() {
    return lineCounter.lineCount();
  }

  public List<E> getAll() {
    Preconditions.checkState(conflationFinished, "Conflation not finished");
    return operationsCommitedToTheConflation();
  }

  public Stream<E> stream() {
    return getAll().stream();
  }

  public boolean add(E e) {
    lineCounter.add(e.lineCount());
    return operationsInTransactionBundle().add(e);
  }

  public boolean addAll(@NotNull Collection<? extends E> c) {
    boolean r = false;
    for (var x : c) {
      r |= this.add(x);
    }
    return r;
  }

  public void clear() {
    operationsCommitedToTheConflation().clear();
    operationsInTransactionBundle().clear();
    lineCounter.clear();
  }

  public void finishConflation() {
    conflationFinished = true;
    operationsCommitedToTheConflation().addAll(operationsInTransactionBundle());
    operationsInTransactionBundle().clear();
    lineCounter.commitTransactionBundle(); // this is not mandatory but it is more consistent
  }
}

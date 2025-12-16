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

import static net.consensys.linea.zktracer.container.stacked.ModuleOperationAdder.existingOperation;
import static net.consensys.linea.zktracer.container.stacked.ModuleOperationAdder.newOperation;

import com.google.common.base.Preconditions;
import java.util.*;
import java.util.stream.Stream;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.ModuleOperation;

/**
 * Implements a system of pseudo-stacked squashed sets where {@link
 * ModuleOperationStackedSet#operationsCommitedToTheConflation()} represents the set of all
 * operations since the beginning of the conflation and {@link
 * ModuleOperationStackedSet#operationsInTransactionBundle()} represents the operations added by the
 * last transaction. We can pop only the operations added by last transaction. The line counting is
 * done by a separate {@link CountOnlyOperation}.
 *
 * @param <E> the type of elements stored in the set
 */
@Accessors(fluent = true)
public class ModuleOperationStackedSet<E extends ModuleOperation> extends StackedSet<E> {
  private final CountOnlyOperation lineCounter = new CountOnlyOperation();
  @Getter private boolean conflationFinished = false;

  public ModuleOperationStackedSet() {
    super();
  }

  /** Prefer this constructor as we preallocate more needed memory */
  public ModuleOperationStackedSet(
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

  public Set<E> getAll() {
    Preconditions.checkState(conflationFinished, "Conflation not finished");
    return operationsCommitedToTheConflation().keySet();
  }

  public boolean isEmpty() {
    return size() == 0;
  }

  public boolean contains(Object o) {
    return operationsInTransactionBundle().containsKey(o)
        || operationsCommitedToTheConflation().containsKey(o);
  }

  public boolean add(E e) {
    if (!operationsCommitedToTheConflation().containsKey(e)) {
      final E isNew = operationsInTransactionBundle().putIfAbsent(e, e);
      if (isNew == null) {
        lineCounter.add(e.lineCount());
      }
      return (isNew == null);
    }
    return false;
  }

  public ModuleOperationAdder addAndGet(E e) {
    // First search if the operation is already present
    final E existing = operationsInTransactionBundle().get(e);
    if (existing != null) return existingOperation(existing);

    final E existingInCommitted = operationsCommitedToTheConflation().get(e);
    if (existingInCommitted != null) return existingOperation(existingInCommitted);

    // Not found, add it
    operationsInTransactionBundle().put(e, e);
    lineCounter.add(e.lineCount());
    return newOperation(e);
  }

  public void clear() {
    operationsCommitedToTheConflation().clear();
    operationsInTransactionBundle().clear();
    lineCounter.clear();
  }

  public void finishConflation() {
    conflationFinished = true;
    operationsCommitedToTheConflation().putAll(operationsInTransactionBundle());
    operationsInTransactionBundle().clear();
    lineCounter.commitTransactionBundle(); // this is not mandatory but it is more consistent
  }

  public List<E> sortOperations(Comparator<E> comparator) {
    final List<E> sortedOperations = new ArrayList<>(getAll());
    sortedOperations.sort(comparator);
    return sortedOperations;
  }

  public Stream<E> stream() {
    return this.getAll().stream();
  }
}

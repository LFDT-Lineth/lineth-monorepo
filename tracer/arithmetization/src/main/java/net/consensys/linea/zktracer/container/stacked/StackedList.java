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

import java.util.ArrayList;
import java.util.List;
import lombok.Getter;
import lombok.experimental.Accessors;

@Accessors(fluent = true)
@Getter
public class StackedList<E> {
  private final List<E> operationsCommitedToTheConflation;
  @Getter private final List<E> operationsInTransactionBundle;

  public StackedList() {
    this.operationsCommitedToTheConflation = new ArrayList<E>();
    this.operationsInTransactionBundle = new ArrayList<E>();
  }

  /** Prefer this constructor as we preallocate more needed memory */
  public StackedList(
      final int expectedConflationNumberOperations, final int expectedTransactionNumberOperations) {
    operationsCommitedToTheConflation = new ArrayList<>(expectedConflationNumberOperations);
    operationsInTransactionBundle = new ArrayList<>(expectedTransactionNumberOperations);
  }

  public void commitTransactionBundle() {
    operationsCommitedToTheConflation.addAll(operationsInTransactionBundle);
    operationsInTransactionBundle.clear();
  }

  public void popTransactionBundle() {
    operationsInTransactionBundle.clear();
  }

  public E getFirst() {
    return operationsCommitedToTheConflation.isEmpty()
        ? operationsInTransactionBundle.getFirst()
        : operationsCommitedToTheConflation.getFirst();
  }

  public E getLast() {
    return operationsInTransactionBundle().isEmpty()
        ? operationsCommitedToTheConflation().getLast()
        : operationsInTransactionBundle().getLast();
  }

  public int size() {
    return operationsInTransactionBundle().size() + operationsCommitedToTheConflation().size();
  }

  public E get(int index) {
    if (index < operationsCommitedToTheConflation().size()) {
      return operationsCommitedToTheConflation().get(index);
    } else {
      return operationsInTransactionBundle()
          .get(index - operationsCommitedToTheConflation().size());
    }
  }

  public List<E> getAll() {
    List<E> all = new ArrayList<>(operationsCommitedToTheConflation());
    all.addAll(operationsInTransactionBundle());
    return all;
  }

  public boolean isEmpty() {
    return size() == 0;
  }

  public boolean add(E e) {
    return operationsInTransactionBundle().add(e);
  }
}

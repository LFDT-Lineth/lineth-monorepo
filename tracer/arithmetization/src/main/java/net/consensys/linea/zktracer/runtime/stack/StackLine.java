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

package net.consensys.linea.zktracer.runtime.stack;

import static net.consensys.linea.zktracer.runtime.stack.StackItem.*;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.Hub;
import org.apache.tuweni.bytes.Bytes;

/**
 * As the zkEVM spec can only handle up to four stack operations per trace line of the {@link Hub},
 * operations on the stack must be decomposed in “lines” (mapping 1-to-1 with a trace line from the
 * hub, hence the name) of zero to four atomic {@link StackItem}.
 */
@Accessors(fluent = true)
public final class StackLine {
  private final List<IndexedStackOperation> items;
  @Getter private final int resultColumn;

  /**
   * @param items zero to four stack operations contained within this line
   * @param resultColumn if positive, in which item to store the expected retroactive result
   */
  public StackLine(List<IndexedStackOperation> items, int resultColumn) {
    this.items = items;
    this.resultColumn = resultColumn;
  }

  /** The default constructor, an empty stack line. */
  public StackLine() {
    this(new ArrayList<>(2), -1);
  }

  /**
   * Build a stack line from a set of {@link StackItem}.
   *
   * @param ct the index of this line within the parent {@link StackContext}
   * @param items the {@link IndexedStackOperation} to include in this line
   */
  StackLine(int ct, IndexedStackOperation... items) {
    this.items = Arrays.asList(items);
    this.resultColumn = -1;
  }

  /**
   * @return a consolidated 4-elements array of the {@link StackItem} – or no-ops
   */
  public List<StackItem> asStackItems() {
    final StackItem[] r = new StackItem[] {empty(), empty(), empty(), empty()};
    for (IndexedStackOperation item : items) {
      r[item.i()] = item.it();
    }
    return Arrays.asList(r);
  }

  /**
   * Sets the value of a stack item in the line. Used to retroactively set the value of push during
   * the unlatching process.
   *
   * @param i the 1-based stack item to alter
   * @param value the {@link Bytes} to use
   */
  public void setResult(int i, Bytes value) {
    for (var item : items) {
      if (item.i() == i - 1) {
        item.it().value(value);
        return;
      }
    }

    throw new RuntimeException(String.format("Item #%s not found in stack line", i));
  }

  /**
   * Sets the value of stack item <code>resultColumn</code>. Used to retroactively set the value of
   * push during the unlatching process.
   *
   * @param value the {@link Bytes} to use
   */
  public void setResult(Bytes value) {
    if (resultColumn == -1) {
      throw new RuntimeException("Stack line has no result column");
    }
    this.setResult(resultColumn, value);
  }

  /**
   * @return whether an item in this stack line requires a retroactively set value.
   */
  public boolean needsResult() {
    return resultColumn >= 0;
  }
}

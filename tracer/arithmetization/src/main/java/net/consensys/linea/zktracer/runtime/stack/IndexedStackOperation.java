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

/**
 * An operation within a {@link StackLine}. This structure is useful because stack lines may be
 * sparse.
 *
 * @param i the index of the stack item within a stack line -- within [[1, 4]
 * @param it the details of the {@link StackItem} to apply to a column of a stack line
 */
record IndexedStackOperation(int i, StackItem it) {
  /**
   * For the sake of homogeneity with the zkEVM spec, {@param i} is set with 1-based indices.
   * However, these indices are used to index 0-based array; hence this sneaky conversion.
   *
   * @return the actual stack operation index
   */
  public int i() {
    return this.i - 1;
  }
}

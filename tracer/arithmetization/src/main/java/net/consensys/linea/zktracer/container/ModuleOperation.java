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

package net.consensys.linea.zktracer.container;

/**
 * Represents an atomic operation within a module. This class is used to automatically cache the
 * line counts of these operations for each transaction.
 */
public abstract class ModuleOperation {
  /** the number of lines this operation will generate within its module trace */
  private int lineCount = -1;

  protected abstract int computeLineCount();

  public int lineCount() {
    if (this.lineCount == -1) {
      this.lineCount = this.computeLineCount();
    }
    return this.lineCount;
  }
}

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
package net.consensys.linea.sequencer.modulelimit;

import lombok.EqualsAndHashCode;
import lombok.Getter;

/** Represents the result of verifying module line counts against their limits. */
@Getter
@EqualsAndHashCode
public class ModuleLimitsValidationResult {
  private final ModuleLineCountValidator.ModuleLineCountResult result;
  private final String moduleName;
  private final Integer moduleLineCount;
  private final Integer moduleLineLimit;
  private final Integer cumulativeModuleLineCount;
  private final Integer cumulativeModuleLineLimit;

  public static final ModuleLimitsValidationResult VALID =
      new ModuleLimitsValidationResult(
          ModuleLineCountValidator.ModuleLineCountResult.VALID, null, null, null, null, null);

  private ModuleLimitsValidationResult(
      final ModuleLineCountValidator.ModuleLineCountResult result,
      final String moduleName,
      final Integer moduleLineCount,
      final Integer moduleLineLimit,
      final Integer cumulativeModuleLineCount,
      final Integer cumulativeModuleLineLimit) {
    this.result = result;
    this.moduleName = moduleName;
    this.moduleLineCount = moduleLineCount;
    this.moduleLineLimit = moduleLineLimit;
    this.cumulativeModuleLineCount = cumulativeModuleLineCount;
    this.cumulativeModuleLineLimit = cumulativeModuleLineLimit;
  }

  public static ModuleLimitsValidationResult moduleNotDefined(final String moduleName) {
    return new ModuleLimitsValidationResult(
        ModuleLineCountValidator.ModuleLineCountResult.MODULE_NOT_DEFINED,
        moduleName,
        null,
        null,
        null,
        null);
  }

  public static ModuleLimitsValidationResult invalidLineCount(
      final String moduleName, final Integer moduleLineCount) {
    return new ModuleLimitsValidationResult(
        ModuleLineCountValidator.ModuleLineCountResult.INVALID_LINE_COUNT,
        moduleName,
        moduleLineCount,
        null,
        null,
        null);
  }

  public static ModuleLimitsValidationResult txModuleLineCountOverflow(
      final String moduleName,
      final Integer moduleLineCount,
      final Integer moduleLineLimit,
      final Integer cumulativeModuleLineCount,
      final Integer cumulativeModuleLineLimit) {
    return new ModuleLimitsValidationResult(
        ModuleLineCountValidator.ModuleLineCountResult.TX_MODULE_LINE_COUNT_OVERFLOW,
        moduleName,
        moduleLineCount,
        moduleLineLimit,
        cumulativeModuleLineCount,
        cumulativeModuleLineLimit);
  }

  public static ModuleLimitsValidationResult blockModuleLineCountFull(
      final String moduleName,
      final Integer moduleLineCount,
      final Integer moduleLineLimit,
      final Integer cumulativeModuleLineCount,
      final Integer cumulativeModuleLineLimit) {

    return new ModuleLimitsValidationResult(
        ModuleLineCountValidator.ModuleLineCountResult.BLOCK_MODULE_LINE_COUNT_FULL,
        moduleName,
        moduleLineCount,
        moduleLineLimit,
        cumulativeModuleLineCount,
        cumulativeModuleLineLimit);
  }

  @Override
  public String toString() {
    final StringBuilder sb = new StringBuilder(result.name());
    if (moduleName != null) {
      sb.append("[module=").append(moduleName);

      if (moduleLineCount != null) {
        sb.append(",lineCount=").append(moduleLineCount);
        sb.append(",lineLimit=").append(moduleLineLimit);
        sb.append(",cumulativeLineCount=").append(cumulativeModuleLineCount);
        sb.append(",cumulativeLineLimit=").append(cumulativeModuleLineLimit);
      }

      sb.append(']');
    }
    return sb.toString();
  }
}

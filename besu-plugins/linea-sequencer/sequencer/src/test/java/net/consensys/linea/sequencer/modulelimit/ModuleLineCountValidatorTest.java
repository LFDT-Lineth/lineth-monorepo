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

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Map;

import org.junit.jupiter.api.Test;

class ModuleLineCountValidatorTest {

  @Test
  void successfulValidation() {
    final var moduleLineCountValidator =
        new ModuleLineCountValidator(Map.of("MOD1", 1, "MOD2", 2, "MOD3", 3));
    final var lineCountTx = Map.of("MOD1", 1, "MOD2", 1, "MOD3", 1);

    assertThat(moduleLineCountValidator.validate(lineCountTx))
        .isEqualTo(ModuleLimitsValidationResult.VALID);
  }

  @Test
  void failedValidationTransactionOverLimit() {
    final var moduleLineCountValidator =
        new ModuleLineCountValidator(Map.of("MOD1", 1, "MOD2", 2, "MOD3", 3));
    final var lineCountTx = Map.of("MOD1", 3, "MOD2", 2, "MOD3", 3);

    assertThat(moduleLineCountValidator.validate(lineCountTx))
        .isEqualTo(ModuleLimitsValidationResult.txModuleLineCountOverflow("MOD1", 3, 1, 3, 1));
  }

  @Test
  void failedValidationBlockOverLimit() {
    final var moduleLineCountValidator =
        new ModuleLineCountValidator(Map.of("MOD1", 1, "MOD2", 2, "MOD3", 3));
    final var prevLineCountTx = Map.of("MOD1", 1, "MOD2", 1, "MOD3", 1);

    final var lineCountTx = Map.of("MOD1", 1, "MOD2", 3, "MOD3", 3);

    assertThat(moduleLineCountValidator.validate(lineCountTx, prevLineCountTx))
        .isEqualTo(ModuleLimitsValidationResult.blockModuleLineCountFull("MOD2", 2, 2, 3, 2));
  }

  @Test
  void failedValidationModuleNotFound() {
    final var moduleLineCountValidator =
        new ModuleLineCountValidator(Map.of("MOD1", 1, "MOD2", 2, "MOD3", 3));

    final var lineCountTx = Map.of("MOD4", 1, "MOD2", 1, "MOD3", 1);

    assertThat(moduleLineCountValidator.validate(lineCountTx, lineCountTx))
        .isEqualTo(ModuleLimitsValidationResult.moduleNotDefined("MOD4"));
  }

  @Test
  void failedValidationInvalidLineCount() {
    final var moduleLineCountValidator =
        new ModuleLineCountValidator(Map.of("MOD1", 1, "MOD2", 2, "MOD3", 3));

    final var lineCountTx = Map.of("MOD1", 1, "MOD2", -2, "MOD3", 1);

    assertThat(moduleLineCountValidator.validate(lineCountTx, lineCountTx))
        .isEqualTo(ModuleLimitsValidationResult.invalidLineCount("MOD2", -2));
  }
}

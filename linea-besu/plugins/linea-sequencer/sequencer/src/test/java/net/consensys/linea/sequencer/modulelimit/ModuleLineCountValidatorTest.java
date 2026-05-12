/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
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

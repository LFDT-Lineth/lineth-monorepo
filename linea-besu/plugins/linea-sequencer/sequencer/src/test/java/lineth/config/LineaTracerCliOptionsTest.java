/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.config;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatExceptionOfType;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import org.hyperledger.besu.services.PicoCLIOptionsImpl;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import picocli.CommandLine;
import picocli.CommandLine.Command;

class LineaTracerCliOptionsTest {
  private static final String MODULE_LINE_LIMITS_RESOURCE_NAME = "/sequencer/line-limits.toml";

  @Command
  static final class TestLineaBesuCommand {}

  @TempDir static Path tempDir;
  private static Path lineLimitsConfPath;

  private CommandLine commandLine;
  private LineaTracerCliOptions tracerCliOptions;

  @BeforeAll
  static void beforeAll() throws IOException {
    lineLimitsConfPath = tempDir.resolve("line-limits.toml");
    Files.copy(
        LineaTracerCliOptionsTest.class.getResourceAsStream(MODULE_LINE_LIMITS_RESOURCE_NAME),
        lineLimitsConfPath);
  }

  @BeforeEach
  void setUp() {
    commandLine = new CommandLine(new TestLineaBesuCommand());
    tracerCliOptions = LineaTracerCliOptions.create();
    new PicoCLIOptionsImpl(commandLine).addPicoCLIOptions("linea", tracerCliOptions);
  }

  @Test
  void shouldLeaveTracingCutoffUnsetByDefault() {
    parseWithModuleLimits();

    assertThat(tracerCliOptions.toDomainObject().tracingEndTimestamp()).isEmpty();
  }

  @Test
  void shouldAcceptZeroTracingCutoff() {
    parseWithModuleLimits(LineaTracerCliOptions.TRACING_END_TIMESTAMP, "0");

    assertThat(tracerCliOptions.toDomainObject().tracingEndTimestamp()).hasValue(0L);
  }

  @Test
  void shouldRejectNegativeTracingCutoff() {
    parseWithModuleLimits(LineaTracerCliOptions.TRACING_END_TIMESTAMP, "-1");

    assertThatExceptionOfType(IllegalArgumentException.class)
        .isThrownBy(tracerCliOptions::toDomainObject)
        .withMessageContaining("must be zero or greater");
  }

  private void parseWithModuleLimits(final String... additionalArguments) {
    final String[] arguments = new String[additionalArguments.length + 2];
    arguments[0] = LineaTracerCliOptions.MODULE_LIMIT_FILE_PATH;
    arguments[1] = lineLimitsConfPath.toString();
    System.arraycopy(additionalArguments, 0, arguments, 2, additionalArguments.length);
    commandLine.parseArgs(arguments);
  }
}

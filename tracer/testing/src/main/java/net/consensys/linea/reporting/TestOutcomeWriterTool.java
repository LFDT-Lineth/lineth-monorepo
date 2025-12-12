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
package net.consensys.linea.reporting;

import static net.consensys.linea.testing.ExecutionEnvironment.CORSET_VALIDATION_RESULT;

import java.io.FileWriter;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.ConcurrentSkipListSet;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.json.JsonConverter;
import org.opentest4j.AssertionFailedError;

@Slf4j
public class TestOutcomeWriterTool {

  private static final String ASSERTION_FAILED = "ASSERTION_FAILED";
  private static final String UNCATEGORIZED_EXCEPTION = "UNCATEGORIZED_EXCEPTION";
  public static JsonConverter jsonConverter = JsonConverter.builder().build();
  private static ObjectMapper objectMapper = new ObjectMapper();

  private static volatile AtomicInteger failedCounter = new AtomicInteger(0);
  private static volatile AtomicInteger successCounter = new AtomicInteger(0);
  private static volatile AtomicInteger disabledCounter = new AtomicInteger(0);
  private static volatile AtomicInteger abortedCounter = new AtomicInteger(0);
  private static volatile ConcurrentMap<
          String, ConcurrentMap<String, ConcurrentSkipListSet<String>>>
      modulesToConstraintsToTests = new ConcurrentHashMap<>(20);

  public static void addFailure(String type, String cause, String test) {
    failedCounter.incrementAndGet();
    modulesToConstraintsToTests
        .computeIfAbsent(type, t -> new ConcurrentHashMap<>())
        .computeIfAbsent(cause, t -> new ConcurrentSkipListSet<>())
        .add(test);
  }

  public static void addSuccess() {
    successCounter.incrementAndGet();
  }

  public static void addAborted() {
    abortedCounter.incrementAndGet();
  }

  public static void addSkipped() {
    disabledCounter.incrementAndGet();
  }

  public static void writeToJsonFile(String name) {
    String fileDirectory = getFileDirectory();
    log.info("writing results summary to {}", fileDirectory + "/" + name);
    try {
      Files.createDirectories(Path.of(fileDirectory));
    } catch (IOException e) {
      log.error("Error - Failed to create test directory output: %s".formatted(e.getMessage()));
      throw new RuntimeException(e);
    }
    try (FileWriter file = new FileWriter(Path.of(fileDirectory, name).toString())) {
      objectMapper.writeValue(
          file,
          new TestOutcome(
              failedCounter.get(),
              successCounter.get(),
              disabledCounter.get(),
              abortedCounter.get(),
              modulesToConstraintsToTests));
    } catch (Exception e) {
      log.error("Error - Failed to write test output: %s".formatted(e.getMessage()));
    }
  }

  public static String getFileDirectory() {
    String jsonDirectory = System.getenv("FAILED_TEST_JSON_DIRECTORY");
    if (jsonDirectory == null || jsonDirectory.isEmpty()) {
      return "../tmp/local/";
    }
    return jsonDirectory;
  }

  public static TestOutcome parseTestOutcome(String jsonString) {
    if (!jsonString.isEmpty()) {
      TestOutcome blockchainReferenceTestOutcome =
          jsonConverter.fromJson(jsonString, TestOutcome.class);
      return blockchainReferenceTestOutcome;
    }
    throw new RuntimeException("invalid JSON");
  }

  public static Map<String, Set<String>> extractConstraints(String message) {
    Map<String, Set<String>> pairs = new HashMap<>();
    String cleaned = message.replaceAll(("\\[[0-9]+m"), " ").replace(']', ' ').trim();

    // case where corset sends constraint failed and the list of constraints
    if (message.contains("constraints failed:")) {
      List<String> lines = cleaned.lines().toList();
      for (String line : lines) {
        if (line.contains("constraints failed:")) {
          String[] failingConstraints =
              line.substring(line.indexOf("constraints failed:") + "constraints failed:".length())
                  .split(",");
          for (String constraint : failingConstraints) {
            getPairFromString(constraint, pairs);
          }
        }
      }
    } else if (message.contains("failing constraint")) {
      // case where corset sends failing constraint with constraints one by one
      List<String> lines = cleaned.lines().toList();
      for (String line : lines) {
        if (line.contains("failing constraint")) {
          String failingConstraints =
              line.substring(line.indexOf("failing constraint") + "failing constraint".length())
                  .replace(':', ' ');

          getPairFromString(failingConstraints, pairs);
        }
      }
    } else {
      // case where corset can't expend the trace
      if (message.contains("Error: while expanding ")) {
        List<String> lines = cleaned.lines().toList();
        for (String line : lines) {
          if (line.contains("reading data for")) {
            String regex = "for\\s+(\\w+)\\s+\\.\\s+(\\w+)";
            Pattern pattern = Pattern.compile(regex);
            Matcher matcher = pattern.matcher(line);
            if (matcher.find()) {
              String module = matcher.group(1);
              String constraint = matcher.group(2);
              pairs
                  .computeIfAbsent("Expanding " + module.trim(), p -> new HashSet<>())
                  .add(constraint.trim());
            } else {
              pairs.computeIfAbsent("UNKNOWN", p -> new HashSet<>()).add("UNKNOWN");
            }
          }
        }
      } else if (message.contains("lookup ")) {
        String regex = "\"lookup \"(.*)\" failed";
        Pattern pattern = Pattern.compile(regex);
        Matcher matcher = pattern.matcher(message);
        if (matcher.find()) {
          String module = "LOOKUP";
          String constraint = matcher.group(1);
          pairs.computeIfAbsent(module.trim(), p -> new HashSet<>()).add(constraint.trim());
        }
      } else if (message.contains("out-of-bounds")) {
        String regex = "column (.*) is out-of-bounds";
        Pattern pattern = Pattern.compile(regex);
        Matcher matcher = pattern.matcher(message);
        if (matcher.find()) {
          String[] group = matcher.group(1).split("\\.");
          String module = group[0];
          String constraint = group[1];
          pairs.computeIfAbsent(module.trim(), p -> new HashSet<>()).add(constraint.trim());
        }
      } else {
        log.info("can't extract constraint, setting UNKNOWN for {}", message);
        pairs.computeIfAbsent("UNKNOWN", p -> new HashSet<>()).add("UNKNOWN");
      }
    }
    return pairs;
  }

  private static void getPairFromString(String constraint, Map<String, Set<String>> pairs) {
    String[] pair;
    if (constraint.contains("-into-")) {
      pair = constraint.split("-into-");
    } else if (constraint.contains("---")) {
      pair = constraint.split("---");
    } else {
      pair = constraint.split("\\.");
    }
    pairs.computeIfAbsent(pair[0].trim(), p -> new HashSet<>()).add(pair[1].trim());
  }

  public static Map<String, Set<String>> getLogEventMessages(Throwable cause) {
    Map<String, Set<String>> logEventMessages = new HashMap<>();
    if (cause != null) {
      if (cause instanceof AssertionFailedError) {
        if (((AssertionFailedError) cause).getActual() != null) {
          if (cause.getMessage().contains(CORSET_VALIDATION_RESULT)) {
            String constraints = cause.getMessage().replaceFirst(CORSET_VALIDATION_RESULT, "");
            logEventMessages = extractConstraints(constraints);
          } else {
            logEventMessages.put(ASSERTION_FAILED, Set.of(formatAssertionError(cause)));
          }
        } else {
          logEventMessages.put(getCauseKey(cause), Set.of(cause.getMessage().split("\n")[0]));
        }
      } else {
        logEventMessages.put(getCauseKey(cause), getValue(cause));
      }
    }
    return logEventMessages;
  }

  public static void mapAndStoreTestResult(
      String testName, TestState success, Map<String, Set<String>> failedConstraints) {
    switch (success) {
      case FAILED -> {
        for (Map.Entry<String, Set<String>> failedConstraint : failedConstraints.entrySet()) {
          String moduleName = failedConstraint.getKey();
          for (String constraint : failedConstraint.getValue()) {
            addFailure(moduleName, constraint, testName);
          }
        }
      }
      case SUCCESS -> TestOutcomeWriterTool.addSuccess();
      case ABORTED -> TestOutcomeWriterTool.addAborted();
      case DISABLED -> TestOutcomeWriterTool.addSkipped();
    }
  }

  private static String formatAssertionError(Throwable cause) {
    return cause
        .getMessage()
        .replaceAll("\n", "")
        .substring(0, Math.min(100, cause.getMessage().length() - 3));
  }

  private static String getCauseKey(Throwable cause) {
    if (cause.getCause() != null) {
      return getCauseKey(cause.getCause());
    }
    if (cause != null) {
      return cause.getClass().getSimpleName();
    }
    return UNCATEGORIZED_EXCEPTION;
  }

  private static Set<String> getValue(Throwable cause) {

    if (cause.getMessage() != null) {
      String[] lines = cause.getMessage().split("\n");
      if (lines[0].isEmpty() && lines.length > 1) {
        return Set.of(lines[1] + " " + firstLineaClassIfPresent(lines));
      }
      return Set.of(lines[0]);
    } else if (cause.getStackTrace() != null) {
      StackTraceElement[] lines = cause.getStackTrace();
      for (int i = 0; i < lines.length; i++) {
        if (lines[i].getClassName().contains("linea")) {
          return Set.of(lines[i].toString());
        }
      }
    }
    return Set.of(UNCATEGORIZED_EXCEPTION);
  }

  private static String firstLineaClassIfPresent(String[] lines) {
    for (int i = 2; i < lines.length; i++) {
      if (lines[i].contains("linea")) {
        return lines[i];
      }
    }
    return "";
  }
}

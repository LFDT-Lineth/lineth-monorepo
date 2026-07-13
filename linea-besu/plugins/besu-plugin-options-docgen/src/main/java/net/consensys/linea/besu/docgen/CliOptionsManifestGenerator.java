/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.besu.docgen;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.lang.reflect.Modifier;
import java.math.BigDecimal;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import picocli.CommandLine;

/**
 * Reflects Linea-Besu plugin {@code *CliOptions} classes via picocli's
 * {@link CommandLine.Model.CommandSpec} and writes the JSON manifest consumed by
 * {@code scripts/besu-plugin-options} MDX rendering.
 *
 * <p>Defaults come from the JVM / picocli model ({@code OptionSpec.getValue()} after
 * {@code create()}), not from a hand-rolled Java expression evaluator.
 */
public final class CliOptionsManifestGenerator {
  private static final String PLUGIN_FLAG_PREFIX = "--plugin-";
  private static final Set<String> EXCLUDED_CLASSES = Set.of("LineaForcedTransactionCliOptions");

  private static final Map<String, String> TITLE_ACRONYMS =
      Map.of(
          "Rpc", "RPC",
          "Tx", "Tx",
          "Tls", "TLS",
          "Url", "URL",
          "L1", "L1",
          "L2", "L2");

  private static final List<PluginSource> PLUGIN_SOURCES =
      List.of(
          new PluginSource(
              "sequencer",
              "Sequencer",
              "linea-besu/plugins/linea-sequencer",
              List.of(
                  "net.consensys.linea.config.LineaBundleCliOptions",
                  "net.consensys.linea.config.LineaForcedTransactionCliOptions",
                  "net.consensys.linea.config.LineaLivenessServiceCliOptions",
                  "net.consensys.linea.config.LineaProfitabilityCliOptions",
                  "net.consensys.linea.config.LineaRejectedTxReportingCliOptions",
                  "net.consensys.linea.config.LineaRpcCliOptions",
                  "net.consensys.linea.config.LineaTracerCliOptions",
                  "net.consensys.linea.config.LineaTransactionPoolValidatorCliOptions",
                  "net.consensys.linea.config.LineaTransactionSelectorCliOptions",
                  "net.consensys.linea.config.LineaTransactionValidatorCliOptions")),
          new PluginSource(
              "tracer",
              "Tracer",
              "tracer/arithmetization",
              List.of(
                  "net.consensys.linea.plugins.config.LineaL1L2BridgeSharedCliOptions",
                  "net.consensys.linea.plugins.config.LineaTracerSharedCliOptions",
                  "net.consensys.linea.plugins.readiness.TracerReadinessCliOptions",
                  "net.consensys.linea.plugins.rpc.RpcCliOptions",
                  "net.consensys.linea.plugins.rpc.tracegeneration.TracesEndpointCliOptions")),
          new PluginSource(
              "state-recovery", "State recovery", "linea-besu/plugins/state-recovery", List.of()));

  private CliOptionsManifestGenerator() {}

  public static void main(final String[] args) throws Exception {
    Path manifestPath = null;
    Path reportPath = null;
    for (int i = 0; i < args.length; i++) {
      if ("--manifest".equals(args[i]) && i + 1 < args.length) {
        manifestPath = Path.of(args[++i]);
      } else if ("--report".equals(args[i]) && i + 1 < args.length) {
        reportPath = Path.of(args[++i]);
      }
    }
    if (manifestPath == null || reportPath == null) {
      System.err.println(
          "Usage: CliOptionsManifestGenerator --manifest <path> --report <path>");
      System.exit(2);
    }

    final Extraction extraction = extract();
    final ObjectMapper mapper =
        new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT).findAndRegisterModules();

    Files.createDirectories(manifestPath.getParent());
    Files.createDirectories(reportPath.getParent());
    mapper.writeValue(manifestPath.toFile(), extraction.manifest());
    mapper.writeValue(reportPath.toFile(), extraction.report());

    final Map<String, Object> counts = castMap(extraction.manifest().get("counts"));
    System.out.printf(
        "Generated %s plugin options across %s plugins / %s groups (%s standard, %s advanced); %s option(s) in %s excluded group(s).%n",
        counts.get("total"),
        counts.get("plugins"),
        counts.get("groups"),
        counts.get("standard"),
        counts.get("advanced"),
        counts.get("excludedOptions"),
        counts.get("excludedGroups"));
    System.out.println("  manifest: " + manifestPath);
    System.out.println("  report:   " + reportPath);
  }

  static Extraction extract() throws Exception {
    final List<Map<String, Object>> pluginsOut = new ArrayList<>();
    final List<Map<String, Object>> optionsOut = new ArrayList<>();
    final List<Map<String, Object>> excludedGroups = new ArrayList<>();
    final List<Map<String, Object>> perPlugin = new ArrayList<>();
    final List<Map<String, Object>> unresolvedDefaults = new ArrayList<>();
    final List<Map<String, Object>> unresolvedTokens = new ArrayList<>();
    final List<Map<String, Object>> missingDescriptions = new ArrayList<>();
    final List<Map<String, Object>> pluginsWithoutOptions = new ArrayList<>();

    for (final PluginSource source : PLUGIN_SOURCES) {
      final List<Map<String, Object>> classesOut = new ArrayList<>();
      int standard = 0;
      int advanced = 0;

      for (final String className : source.classNames()) {
        final Class<?> clazz = Class.forName(className);
        final String simpleName = clazz.getSimpleName();
        final Object instance = instantiate(clazz);
        final String configKey = readConfigKey(clazz);
        final List<OptionRecord> options = readOptions(clazz, instance);

        final List<OptionRecord> inScope =
            options.stream()
                .filter(o -> !o.names().isEmpty() && o.names().getFirst().startsWith(PLUGIN_FLAG_PREFIX))
                .toList();

        if (inScope.isEmpty()) {
          continue;
        }

        if (EXCLUDED_CLASSES.contains(simpleName)) {
          final Map<String, Object> excluded = new LinkedHashMap<>();
          excluded.put("plugin", source.key());
          excluded.put("className", simpleName);
          excluded.put("configKey", configKey);
          excluded.put("optionCount", inScope.size());
          excluded.put(
              "reason", "Unreleased feature (forced transactions). TODO: include once shipped.");
          excludedGroups.add(excluded);
          continue;
        }

        final String classTitle = titleFromClassName(simpleName);
        for (final OptionRecord option : inScope) {
          if (option.hidden()) {
            advanced++;
          } else {
            standard++;
          }
          if (option.descriptionRaw() == null || option.descriptionRaw().isBlank()) {
            final Map<String, Object> missing = new LinkedHashMap<>();
            missing.put("plugin", source.key());
            missing.put("option", option.names().getFirst());
            missing.put("sourceFile", simpleName + ".java");
            missingDescriptions.add(missing);
          }
          if (!option.defaultResolved()) {
            final Map<String, Object> unresolved = new LinkedHashMap<>();
            unresolved.put("plugin", source.key());
            unresolved.put("option", option.names().getFirst());
            unresolved.put("sourceFile", simpleName + ".java");
            unresolvedDefaults.add(unresolved);
          }
          if (option.description().contains("${DEFAULT-VALUE}")) {
            final Map<String, Object> token = new LinkedHashMap<>();
            token.put("plugin", source.key());
            token.put("option", option.names().getFirst());
            token.put("token", "${DEFAULT-VALUE}");
            unresolvedTokens.add(token);
          }
          if (option.description().contains("${COMPLETION-CANDIDATES}")) {
            final Map<String, Object> token = new LinkedHashMap<>();
            token.put("plugin", source.key());
            token.put("option", option.names().getFirst());
            token.put("token", "${COMPLETION-CANDIDATES}");
            unresolvedTokens.add(token);
          }

          final Map<String, Object> flat = new LinkedHashMap<>();
          flat.put("group", configKey);
          flat.put("configKey", configKey);
          flat.put("sourceFile", simpleName + ".java");
          flat.put("sourceLine", option.sourceLine());
          flat.put("names", option.names());
          flat.put("description", option.description());
          flat.put("descriptionRaw", option.descriptionRaw());
          flat.put("default", option.defaultDisplay());
          flat.put("defaultResolved", option.defaultResolved());
          flat.put("type", option.type());
          flat.put("paramLabel", option.paramLabel());
          flat.put("javaType", option.javaType());
          flat.put("hidden", option.hidden());
          flat.put("plugin", source.key());
          flat.put("pluginTitle", source.title());
          flat.put("className", simpleName);
          flat.put("classTitle", classTitle);
          optionsOut.add(flat);
        }

        final Map<String, Object> classEntry = new LinkedHashMap<>();
        classEntry.put("className", simpleName);
        classEntry.put("title", classTitle);
        classEntry.put("configKey", configKey);
        classEntry.put("optionCount", inScope.size());
        classesOut.add(classEntry);
      }

      classesOut.sort(Comparator.comparing(c -> String.valueOf(c.get("className"))));

      final boolean hasOptions = !classesOut.isEmpty();
      if (!hasOptions) {
        final Map<String, Object> empty = new LinkedHashMap<>();
        empty.put("plugin", source.key());
        empty.put("note", "No plugin-specific CLI options found.");
        pluginsWithoutOptions.add(empty);
      }

      final Map<String, Object> pluginEntry = new LinkedHashMap<>();
      pluginEntry.put("key", source.key());
      pluginEntry.put("title", source.title());
      pluginEntry.put("root", source.root());
      pluginEntry.put("hasOptions", hasOptions);
      pluginEntry.put("classes", classesOut);
      pluginsOut.add(pluginEntry);

      final Map<String, Object> breakdown = new LinkedHashMap<>();
      breakdown.put("plugin", source.key());
      breakdown.put("title", source.title());
      breakdown.put("standard", standard);
      breakdown.put("advanced", advanced);
      breakdown.put("total", standard + advanced);
      breakdown.put("classes", classesOut.size());
      breakdown.put("hasOptions", hasOptions);
      perPlugin.add(breakdown);
    }

    final int hidden = (int) optionsOut.stream().filter(o -> Boolean.TRUE.equals(o.get("hidden"))).count();
    final int groups = pluginsOut.stream().mapToInt(p -> ((List<?>) p.get("classes")).size()).sum();
    final int excludedOptions =
        excludedGroups.stream().mapToInt(g -> ((Number) g.get("optionCount")).intValue()).sum();

    final Map<String, Object> counts = new LinkedHashMap<>();
    counts.put("plugins", pluginsOut.size());
    counts.put("groups", groups);
    counts.put("total", optionsOut.size());
    counts.put("standard", optionsOut.size() - hidden);
    counts.put("advanced", hidden);
    counts.put("hidden", hidden);
    counts.put("rendered", optionsOut.size());
    counts.put("excludedGroups", excludedGroups.size());
    counts.put("excludedOptions", excludedOptions);

    final Map<String, Object> manifest = new LinkedHashMap<>();
    manifest.put("generatedFrom", "linea-monorepo (Linea-Besu plugins)");
    manifest.put(
        "note",
        "Generated by :linea-besu:plugins:besu-plugin-options-docgen. Do not edit by hand.");
    manifest.put(
        "hiddenTreatment",
        "Hidden options are included and marked Advanced (real operator flags not surfaced in CLI help).");
    manifest.put("scope", "Plugin-specific options only (flags starting with --plugin-).");
    manifest.put("counts", counts);
    manifest.put("perPlugin", perPlugin);
    manifest.put("excludedGroups", excludedGroups);
    manifest.put("plugins", pluginsOut);
    manifest.put("options", optionsOut);

    final Map<String, Object> report = new LinkedHashMap<>();
    report.put(
        "hiddenTreatment",
        "Hidden options are included and marked Advanced (real operator flags not surfaced in CLI help).");
    report.put(
        "excludedGroups",
        excludedGroups.stream()
            .map(
                g -> {
                  final Map<String, Object> row = new LinkedHashMap<>();
                  row.put("plugin", g.get("plugin"));
                  row.put("className", g.get("className"));
                  row.put("optionCount", g.get("optionCount"));
                  row.put("reason", "Unreleased feature (forced transactions).");
                  return row;
                })
            .toList());
    report.put("pluginsWithoutOptions", pluginsWithoutOptions);
    report.put("advancedOptionCount", hidden);
    report.put("missingDescriptions", missingDescriptions);
    report.put("unresolvedDefaults", unresolvedDefaults);
    report.put("unresolvedTokens", unresolvedTokens);

    return new Extraction(manifest, report);
  }

  private static Object instantiate(final Class<?> clazz) throws Exception {
    try {
      final Method create = clazz.getMethod("create");
      if (Modifier.isStatic(create.getModifiers())) {
        return create.invoke(null);
      }
    } catch (final NoSuchMethodException ignored) {
      // fall through
    }
    final var ctor = clazz.getDeclaredConstructor();
    ctor.setAccessible(true);
    return ctor.newInstance();
  }

  private static String readConfigKey(final Class<?> clazz) throws Exception {
    try {
      final Field field = clazz.getDeclaredField("CONFIG_KEY");
      field.setAccessible(true);
      final Object value = field.get(null);
      return value == null ? null : String.valueOf(value);
    } catch (final NoSuchFieldException e) {
      return null;
    }
  }

  private static List<OptionRecord> readOptions(final Class<?> clazz, final Object instance) {
    final CommandLine commandLine = new CommandLine(instance);
    final List<OptionRecord> out = new ArrayList<>();

    for (final CommandLine.Model.OptionSpec option : commandLine.getCommandSpec().options()) {
      final List<String> names =
          Arrays.stream(option.names()).map(String::trim).filter(s -> !s.isEmpty()).toList();
      if (names.isEmpty()) {
        continue;
      }

      final String paramLabel = blankToNull(option.paramLabel());
      final Class<?> javaClass = option.type();
      final String javaType = javaClass.getTypeName().replace('$', '.');
      final String type =
          paramLabel != null ? paramLabel.replaceAll("^<|>$", "") : simpleTypeName(javaClass);

      final String descriptionRaw = joinDescriptions(option.description());
      Object rawDefault = null;
      boolean defaultResolved = false;
      try {
        rawDefault = option.getValue();
        defaultResolved = rawDefault != null;
      } catch (final Exception ignored) {
        defaultResolved = false;
      }

      final String annotatedDefault = option.defaultValue();
      if (!defaultResolved
          && annotatedDefault != null
          && !annotatedDefault.isEmpty()
          && !"__no_default_value__".equals(annotatedDefault)) {
        rawDefault = annotatedDefault;
        defaultResolved = true;
      }

      String defaultDisplay = defaultResolved ? formatDefault(rawDefault) : null;
      if ("__no_default_value__".equals(defaultDisplay)) {
        defaultDisplay = null;
        defaultResolved = false;
      }

      String description = descriptionRaw;
      if (description.contains("${DEFAULT-VALUE}") && defaultDisplay != null) {
        description = description.replace("${DEFAULT-VALUE}", defaultDisplay);
      }
      // Docs have a Default column; drop picocli-help "(default: …)" from the description text.
      description = stripEmbeddedDefault(description);

      out.add(
          new OptionRecord(
              names,
              description,
              descriptionRaw,
              defaultDisplay,
              defaultResolved,
              type,
              paramLabel,
              javaType,
              option.hidden(),
              -1));
    }
    return out;
  }

  private static String joinDescriptions(final String[] parts) {
    if (parts == null || parts.length == 0) {
      return "";
    }
    return String.join(" ", parts).trim();
  }

  /**
   * Remove {@code (default: …)} parentheticals that picocli embeds for {@code --help}. Docs already
   * expose defaults in a dedicated column.
   */
  private static String stripEmbeddedDefault(final String description) {
    if (description == null || description.isBlank()) {
      return description == null ? "" : description;
    }
    return description
        .replaceAll("(?i)\\s*\\(default:\\s*[^)]*\\)", "")
        .replaceAll("\\s{2,}", " ")
        .trim();
  }

  private static String formatDefault(final Object value) {
    if (value == null) {
      return null;
    }
    if (value instanceof String
        || value instanceof Boolean
        || value instanceof Number
        || value instanceof BigDecimal) {
      return String.valueOf(value);
    }
    if (value instanceof final Collection<?> collection) {
      if (collection.isEmpty()) {
        return "[]";
      }
      return collection.stream().map(CliOptionsManifestGenerator::formatDefault).toList().toString();
    }
    if (value.getClass().isArray()) {
      final int len = java.lang.reflect.Array.getLength(value);
      if (len == 0) {
        return "[]";
      }
      final List<String> items = new ArrayList<>(len);
      for (int i = 0; i < len; i++) {
        items.add(formatDefault(java.lang.reflect.Array.get(value, i)));
      }
      return items.toString();
    }
    // Addresses / Bytes / enums → stable toString; leave blank if it looks unusable.
    final String text = String.valueOf(value);
    if (text.startsWith(value.getClass().getName() + "@")) {
      return null;
    }
    return text;
  }

  private static String simpleTypeName(final Class<?> type) {
    if (type.isArray()) {
      return simpleTypeName(type.getComponentType()) + "[]";
    }
    return type.getSimpleName();
  }

  private static String titleFromClassName(final String className) {
    String base = className.replaceFirst("^Linea", "").replaceFirst("CliOptions$", "");
    final Matcher matcher = Pattern.compile("[A-Z][a-z0-9]*").matcher(base);
    final List<String> words = new ArrayList<>();
    while (matcher.find()) {
      words.add(matcher.group());
    }
    if (words.isEmpty()) {
      return base;
    }
    final List<String> titled = new ArrayList<>();
    for (int i = 0; i < words.size(); i++) {
      final String w = words.get(i);
      if (TITLE_ACRONYMS.containsKey(w)) {
        titled.add(TITLE_ACRONYMS.get(w));
      } else {
        titled.add(i == 0 ? w : w.toLowerCase());
      }
    }
    return String.join(" ", titled).trim();
  }

  private static String blankToNull(final String s) {
    return s == null || s.isBlank() ? null : s;
  }

  @SuppressWarnings("unchecked")
  private static Map<String, Object> castMap(final Object value) {
    return (Map<String, Object>) value;
  }

  private record PluginSource(String key, String title, String root, List<String> classNames) {}

  private record OptionRecord(
      List<String> names,
      String description,
      String descriptionRaw,
      String defaultDisplay,
      boolean defaultResolved,
      String type,
      String paramLabel,
      String javaType,
      boolean hidden,
      int sourceLine) {}

  record Extraction(Map<String, Object> manifest, Map<String, Object> report) {}
}

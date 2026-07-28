/**
 * Drift + completeness check for Postman config docs.
 * Re-generates MDX partials in memory, formats with prettier, and compares
 * against committed files.
 *
 * Run: pnpm --filter @lfdt-lineth/postman run docs:check
 */

import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import prettier from "prettier";

import { generatePartials } from "./generate-config-docs";

const OUTPUT_DIR = path.join(__dirname, "..", "docs", "_generated", "postman");
const WRAPPER_PATH = path.join(__dirname, "..", "docs", "linea-postman-options.mdx");

async function formatWith(text: string, ext: string): Promise<string> {
  const config = (await prettier.resolveConfig(`output.${ext}`)) ?? {};
  return prettier.format(text, { ...config, filepath: `output.${ext}` });
}

async function check(): Promise<string[]> {
  const failures: string[] = [];
  const { partials, wrapperMarkdown } = generatePartials();

  // Format in-memory output with prettier to match committed (prettier-formatted) files
  const [formattedWrapper, ...formattedPartials] = await Promise.all([
    formatWith(wrapperMarkdown, "mdx"),
    ...partials.map((p) => formatWith(p.markdown, "mdx")),
  ]);
  const formattedPartialsList = partials.map((p, i) => ({ ...p, markdown: formattedPartials[i] }));

  // Collect committed partials (excluding provenance.mdx which is write-only by CI)
  const committedFiles = new Set<string>();
  if (existsSync(OUTPUT_DIR)) {
    for (const entry of readdirSync(OUTPUT_DIR)) {
      if (entry.endsWith(".mdx") && entry !== "provenance.mdx") {
        committedFiles.add(entry);
      }
    }
  }

  const generatedNames = new Set(formattedPartialsList.map((p) => `${p.sectionId}.mdx`));

  // Check each generated partial against committed file
  for (const p of formattedPartialsList) {
    const filename = `${p.sectionId}.mdx`;
    const filepath = path.join(OUTPUT_DIR, filename);
    if (!existsSync(filepath)) {
      failures.push(`partial: missing _generated/postman/${filename}`);
      continue;
    }
    const committed = readFileSync(filepath, "utf8");
    if (committed !== p.markdown) {
      failures.push(
        `partial: _generated/postman/${filename} is out of date (run pnpm --filter @lfdt-lineth/postman run docs:generate).`,
      );
    }
  }

  // Check for unexpected committed partials
  for (const filename of committedFiles) {
    if (!generatedNames.has(filename)) {
      failures.push(`partial: unexpected _generated/postman/${filename} (not produced by current sources)`);
    }
  }

  // Check wrapper
  if (!existsSync(WRAPPER_PATH)) {
    failures.push(
      `wrapper: missing template at ${path.relative(process.cwd(), WRAPPER_PATH)} (run docs:generate once).`,
    );
  } else {
    const committedWrapper = readFileSync(WRAPPER_PATH, "utf8");
    if (committedWrapper !== formattedWrapper) {
      failures.push(
        `wrapper: ${path.relative(process.cwd(), WRAPPER_PATH)} is out of date (run pnpm --filter @lfdt-lineth/postman run docs:generate).`,
      );
    }
  }

  return failures;
}

check().then((failures) => {
  if (failures.length) {
    console.error("Postman config docs drift check failed:");
    for (const f of failures) console.error(`- ${f}`);
    process.exit(1);
  }
  console.log("Postman config docs check passed (committed output matches source).");
});

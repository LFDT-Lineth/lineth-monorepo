#!/usr/bin/env ts-node
import { validateAllAddressRegistries } from "../../common/helpers/addressRegistry";

function main(): void {
  const issues = validateAllAddressRegistries();

  if (issues.length === 0) {
    console.log("Address registry validation passed.");
    return;
  }

  console.error(`Address registry validation failed with ${issues.length} issue(s):\n`);
  for (const issue of issues) {
    console.error(`- ${issue.file} :: ${issue.path} :: ${issue.message}`);
  }
  process.exitCode = 1;
}

main();

import { execSync } from "child_process";

export function getGitBranch(): string {
  return execSync("git rev-parse --abbrev-ref HEAD").toString().trim();
}

export function getGitCommitHash(): string {
  return execSync(`git rev-parse HEAD`).toString().trim();
}

export function getGitTagsAtCommitHash(): string[] {
  const tags = execSync(`git tag --points-at ${getGitCommitHash()}`).toString().trim();
  return tags.length > 0 ? tags.split("\n") : [];
}

export function validateDeployBranchAndTags(networkName: string) {
  const tagPattern = /^contract-audit-(\S+-)?\d{4}-\d{2}-\d{2}$/;
  const networksRequiringAuditedCode: string[] = ["mainnet", "linea_mainnet", "sepolia", "linea_sepolia"];

  console.log("Validating if the network to deploy to requires an audited version.");

  if (networksRequiringAuditedCode.includes(networkName)) {
    const tags = getGitTagsAtCommitHash();

    if (!tags.some((value) => tagPattern.test(value))) {
      throw new Error("Tags for this branch are missing. Format 'contract-audit-FIRM-DATE' or 'contract-audit-DATE");
    }

    // If the code passes validation, check if compilation is needed
    console.log("Forcing contract compilation based on validation logic...");
    try {
      execSync("ENABLE_VIA_IR=true pnpm exec hardhat compile --force", { stdio: "inherit" });
    } catch (error) {
      console.error("Compilation failed", error);
      throw new Error("Failed to compile contracts.");
    }
  }
}

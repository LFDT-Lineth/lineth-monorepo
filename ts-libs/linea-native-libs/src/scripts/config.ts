import { config } from "dotenv";

config();

/** Targets named as `<os.platform()>-<os.arch()>`, matching the folders the runtime loads from. */
export const SUPPORTED_TARGETS = ["darwin-arm64", "darwin-x64", "linux-arm64", "linux-x64"] as const;

export type Target = (typeof SUPPORTED_TARGETS)[number];

type BuildConfig = {
  nativeLibReleaseTag: string;
  targets: Target[];
};

function isSupportedTarget(target: string): target is Target {
  return (SUPPORTED_TARGETS as readonly string[]).includes(target);
}

/**
 * Resolves which platform binaries to keep.
 * `NATIVE_LIBS_TARGETS` accepts a comma-separated list of targets, or `all`.
 * When unset, only the binary for the machine running the build is kept.
 */
function getTargets(): Target[] {
  const rawTargets = process.env.NATIVE_LIBS_TARGETS?.trim();

  if (rawTargets === "all") {
    return [...SUPPORTED_TARGETS];
  }

  const requestedTargets = rawTargets
    ? rawTargets
        .split(",")
        .map((target) => target.trim())
        .filter(Boolean)
    : [`${process.platform}-${process.arch}`];

  const unsupportedTargets = requestedTargets.filter((target) => !isSupportedTarget(target));
  if (unsupportedTargets.length > 0) {
    throw new Error(
      `Unsupported native lib target(s): ${unsupportedTargets.join(", ")}. ` +
        `Set NATIVE_LIBS_TARGETS to "all" or a comma-separated list of: ${SUPPORTED_TARGETS.join(", ")}`,
    );
  }

  return [...new Set(requestedTargets as Target[])];
}

export function getBuildConfig(): BuildConfig {
  const nativeLibReleaseTag = process.env.NATIVE_LIBS_RELEASE_TAG;

  if (!nativeLibReleaseTag) {
    throw new Error("NATIVE_LIBS_RELEASE_TAG environment variable is not set");
  }

  return {
    nativeLibReleaseTag,
    targets: getTargets(),
  };
}

"use client";

import { useEffect, useRef } from "react";
import { Disclosure } from "@headlessui/react";
import { useVerifierStore } from "@/stores/verifier";
import { Card } from "@/components/ui/card";
import type { ContractVerificationResult, VerificationStatus } from "@consensys/linea-contract-integrity-verifier";
import styles from "./results-section.module.scss";
import clsx from "clsx";

// ============================================================================
// Info Icon with Tooltip
// ============================================================================

function InfoTooltip({ comment }: { comment: string }) {
  return (
    <span className={styles.tooltipWrapper} title={comment}>
      <svg className={styles.infoIcon} width="14" height="14" viewBox="0 0 20 20" fill="currentColor" aria-label="Info">
        <path
          fillRule="evenodd"
          d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z"
          clipRule="evenodd"
        />
      </svg>
      <span className={styles.tooltip}>{comment}</span>
    </span>
  );
}

const statusIcons: Record<VerificationStatus, string> = {
  pass: "✓",
  fail: "✗",
  warn: "!",
  skip: "○",
};

const statusLabels: Record<VerificationStatus, string> = {
  pass: "Pass",
  fail: "Fail",
  warn: "Warning",
  skip: "Skipped",
};

function StatusBadge({ status }: { status: VerificationStatus }) {
  return (
    <span className={clsx(styles.badge, styles[status])}>
      <span aria-hidden="true">{statusIcons[status]}</span>
      {statusLabels[status]}
    </span>
  );
}

// ============================================================================
// Comment Extraction from Config
// ============================================================================

interface ViewCallComment {
  function: string;
  params?: unknown[];
  comment: string;
}

interface ConfigComments {
  viewCalls: ViewCallComment[]; // Store all view call comments for flexible matching
  slots: Map<string, string>; // key: slot hex
  storagePaths: Map<string, string>; // key: path
}

/**
 * Parse the raw config content to extract $comment fields.
 * We need to parse the raw content string because the typed VerifierConfig
 * doesn't include $comment fields.
 */
function parseRawConfigForComments(rawContent: string, format: string): Record<string, unknown> | null {
  if (format === "markdown") {
    return null; // Markdown doesn't support $comment the same way
  }

  try {
    // Replace unquoted env vars (like chainId: ${VAR}) with 0
    let sanitized = rawContent.replace(/:\s*\$\{([^}]+)\}(\s*[,}\]])/g, ": 0$2");
    // Replace quoted env vars with placeholder strings
    sanitized = sanitized.replace(/"\$\{([^}]+)\}"/g, '"__ENV__"');
    return JSON.parse(sanitized);
  } catch {
    return null;
  }
}

/**
 * Find a comment for a view call by matching function name and optionally params.
 * Falls back to matching just by function name if exact match isn't found.
 */
function findViewCallComment(
  viewCalls: ViewCallComment[],
  functionName: string,
  params: unknown[] | undefined,
): string | undefined {
  // First try exact match by function name and params (excluding __ENV__ placeholders)
  for (const vc of viewCalls) {
    if (vc.function === functionName) {
      // If no params in config or result, match by function name alone
      if (!vc.params?.length && !params?.length) {
        return vc.comment;
      }
      // If params match (ignoring __ENV__ placeholders)
      if (vc.params && params && vc.params.length === params.length) {
        const paramsMatch = vc.params.every((p, i) => {
          if (p === "__ENV__") return true; // Placeholder matches anything
          return String(p).toLowerCase() === String(params[i]).toLowerCase();
        });
        if (paramsMatch) return vc.comment;
      }
    }
  }

  // Fallback: just match by function name (return first match)
  const byName = viewCalls.find((vc) => vc.function === functionName);
  return byName?.comment;
}

function extractCommentsFromConfig(contractName: string, rawContent: string, format: string): ConfigComments {
  const comments: ConfigComments = {
    viewCalls: [],
    slots: new Map(),
    storagePaths: new Map(),
  };

  const rawConfig = parseRawConfigForComments(rawContent, format);
  if (!rawConfig) {
    return comments;
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const contracts = (rawConfig as any).contracts;
  if (!Array.isArray(contracts)) {
    return comments;
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const contract = contracts.find((c: any) => c.name === contractName);
  if (!contract?.stateVerification) {
    return comments;
  }

  const sv = contract.stateVerification;

  // Extract view call comments
  if (Array.isArray(sv.viewCalls)) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    for (const vc of sv.viewCalls) {
      if (vc.$comment && vc.function) {
        comments.viewCalls.push({
          function: vc.function,
          params: vc.params,
          comment: vc.$comment,
        });
      }
    }
  }

  // Extract slot comments
  if (Array.isArray(sv.slots)) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    for (const slot of sv.slots) {
      if (slot.$comment && slot.slot) {
        comments.slots.set(slot.slot, slot.$comment);
      }
    }
  }

  // Extract storage path comments
  if (Array.isArray(sv.storagePaths)) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    for (const sp of sv.storagePaths) {
      if (sp.$comment && sp.path) {
        comments.storagePaths.set(sp.path, sp.$comment);
      }
    }
  }

  return comments;
}

// ============================================================================
// Contract Result Card
// ============================================================================

interface ContractResultCardProps {
  result: ContractVerificationResult;
  verbose: boolean;
  comments: ConfigComments;
}

function ContractResultCard({ result, verbose, comments }: ContractResultCardProps) {
  const { bytecodeResult, abiResult, stateResult, immutableValuesResult, groupedImmutables, error } = result;

  // Determine overall status
  let overallStatus: VerificationStatus = "pass";
  if (error) {
    overallStatus = "fail";
  } else {
    const statuses = [bytecodeResult?.status, abiResult?.status, stateResult?.status].filter(
      Boolean,
    ) as VerificationStatus[];

    if (statuses.includes("fail")) {
      overallStatus = "fail";
    } else if (statuses.includes("warn")) {
      overallStatus = "warn";
    } else if (statuses.every((s) => s === "skip")) {
      overallStatus = "skip";
    }
  }

  return (
    <Disclosure>
      {({ open }) => (
        <div className={clsx(styles.resultCard, styles[overallStatus])}>
          <Disclosure.Button className={styles.resultHeader}>
            <div className={styles.resultInfo}>
              <h3 className={styles.contractName}>{result.contract.name}</h3>
              <div className={styles.resultMeta}>
                <span className={styles.chain}>{result.contract.chain}</span>
                <code className={styles.address}>{result.contract.address}</code>
              </div>
            </div>
            <div className={styles.resultActions}>
              <StatusBadge status={overallStatus} />
              <svg
                className={clsx(styles.chevron, open && styles.open)}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                aria-hidden="true"
              >
                <polyline points="6 9 12 15 18 9" />
              </svg>
            </div>
          </Disclosure.Button>

          <Disclosure.Panel className={styles.resultDetails}>
            {error && (
              <div className={styles.errorSection}>
                <p className={styles.errorMessage}>{error}</p>
              </div>
            )}

            {bytecodeResult && (
              <div className={styles.section}>
                <div className={styles.sectionHeader}>
                  <h4>Bytecode Verification</h4>
                  <StatusBadge status={bytecodeResult.status} />
                </div>
                <p className={styles.sectionMessage}>
                  {bytecodeResult.message}
                  {groupedImmutables && groupedImmutables.some((g) => g.isFragmented) && (
                    <InfoTooltip comment="Fragmented: When an immutable value contains zero bytes (e.g., 0x73bf00ad...), it appears split because zeros match the artifact placeholder." />
                  )}
                </p>
                {verbose && bytecodeResult.matchPercentage !== undefined && (
                  <p className={styles.detail}>Match: {bytecodeResult.matchPercentage.toFixed(2)}%</p>
                )}
                {verbose && bytecodeResult.localBytecodeLength !== undefined && (
                  <p className={styles.detail}>
                    Local bytecode: {bytecodeResult.localBytecodeLength} bytes, Remote bytecode:{" "}
                    {bytecodeResult.remoteBytecodeLength} bytes
                  </p>
                )}

                {/* Named Immutable Values Verification */}
                {immutableValuesResult && immutableValuesResult.results.length > 0 && (
                  <div className={styles.subsection}>
                    <div className={styles.subsectionHeader}>
                      <h5>Named Immutables ({immutableValuesResult.results.length})</h5>
                      <StatusBadge status={immutableValuesResult.status} />
                    </div>
                    <table className={styles.table}>
                      <thead>
                        <tr>
                          <th>Name</th>
                          <th>Expected</th>
                          <th>Actual</th>
                          <th>Status</th>
                        </tr>
                      </thead>
                      <tbody>
                        {immutableValuesResult.results.map((immResult, i) => (
                          <tr key={i} className={immResult.status === "fail" ? styles.mismatchRow : undefined}>
                            <td>
                              <code>{immResult.name}</code>
                            </td>
                            <td>
                              <code className={immResult.status === "fail" ? styles.mismatchValue : undefined}>
                                {immResult.expected ? `${immResult.expected.slice(0, 10)}...${immResult.expected.slice(-8)}` : "-"}
                              </code>
                            </td>
                            <td>
                              <code className={immResult.status === "fail" ? styles.mismatchValue : undefined}>
                                {immResult.actual ? `${immResult.actual.slice(0, 10)}...${immResult.actual.slice(-8)}` : "not found"}
                              </code>
                            </td>
                            <td>
                              <StatusBadge status={immResult.status} />
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}

                {verbose && bytecodeResult.immutableDifferences && bytecodeResult.immutableDifferences.length > 0 && (
                  <div className={styles.detail}>
                    <p>Immutable differences:</p>
                    {groupedImmutables && groupedImmutables.length > 0 ? (
                      <ul className={styles.selectorList}>
                        {groupedImmutables.map((group) => (
                          <li key={group.index}>
                            {group.isFragmented ? (
                              <div className={styles.fragmentedGroup}>
                                <span className={styles.fragmentedLabel}>
                                  {group.index}) Fragmented immutable at position {group.refStart}:
                                </span>
                                <div className={styles.fragmentDetails}>
                                  <span>
                                    Full value: <code>0x{group.fullValue.replace(/^0+/, "") || "0"}</code>
                                  </span>
                                  <ul className={styles.fragmentList}>
                                    {group.fragments.map((frag, fragIdx) => (
                                      <li key={fragIdx}>
                                        {group.index}.{fragIdx + 1}) Position {frag.position}:{" "}
                                        <code>{frag.remoteValue}</code>
                                      </li>
                                    ))}
                                  </ul>
                                </div>
                              </div>
                            ) : (
                              <span>
                                {group.index}) Position {group.fragments[0]?.position}:{" "}
                                <code>0x{group.fullValue.replace(/^0+/, "") || "0"}</code>
                                {group.refLength === 32 && <span> (address)</span>}
                              </span>
                            )}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <ul className={styles.selectorList}>
                        {bytecodeResult.immutableDifferences.map((diff, i) => (
                          <li key={i}>
                            Position {diff.position}: <code>{diff.remoteValue}</code>
                            {diff.possibleType && <span> ({diff.possibleType})</span>}
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                )}
              </div>
            )}

            {abiResult && (
              <div className={styles.section}>
                <div className={styles.sectionHeader}>
                  <h4>ABI Verification</h4>
                  <StatusBadge status={abiResult.status} />
                </div>
                <p className={styles.sectionMessage}>{abiResult.message}</p>
                {verbose && abiResult.localSelectors && (
                  <p className={styles.detail}>Local selectors: {abiResult.localSelectors.length}</p>
                )}
                {verbose && abiResult.remoteSelectors && (
                  <p className={styles.detail}>Remote selectors: {abiResult.remoteSelectors.length}</p>
                )}
                {abiResult.missingSelectors && abiResult.missingSelectors.length > 0 && (
                  <div className={styles.detail}>
                    <p>Missing selectors:</p>
                    <ul className={styles.selectorList}>
                      {abiResult.missingSelectors.map((s) => (
                        <li key={s}>
                          <code>{s}</code>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                {verbose && abiResult.extraSelectors && abiResult.extraSelectors.length > 0 && (
                  <div className={styles.detail}>
                    <p>Extra selectors on chain:</p>
                    <ul className={styles.selectorList}>
                      {abiResult.extraSelectors.map((s) => (
                        <li key={s}>
                          <code>{s}</code>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            )}

            {stateResult && (
              <div className={styles.section}>
                <div className={styles.sectionHeader}>
                  <h4>State Verification</h4>
                  <StatusBadge status={stateResult.status} />
                </div>
                <p className={styles.sectionMessage}>{stateResult.message}</p>

                {/* View Calls - always show table but show params only in verbose */}
                {stateResult.viewCallResults && stateResult.viewCallResults.length > 0 && (
                  <div className={styles.subsection}>
                    <h5>View Calls ({stateResult.viewCallResults.length})</h5>
                    <table className={styles.table}>
                      <thead>
                        <tr>
                          <th>Function</th>
                          {verbose && <th>Params</th>}
                          <th>Expected</th>
                          <th>Actual</th>
                          <th>Status</th>
                        </tr>
                      </thead>
                      <tbody>
                        {stateResult.viewCallResults.map((r, i) => {
                          const comment = findViewCallComment(comments.viewCalls, r.function, r.params);
                          return (
                            <tr key={i}>
                              <td>
                                <span className={styles.functionCell}>
                                  <code>{r.function}</code>
                                  {comment && <InfoTooltip comment={comment} />}
                                </span>
                              </td>
                              {verbose && (
                                <td>
                                  <code>{r.params?.length ? JSON.stringify(r.params) : "()"}</code>
                                </td>
                              )}
                              <td>
                                <code>{String(r.expected)}</code>
                              </td>
                              <td>
                                <code>{String(r.actual)}</code>
                              </td>
                              <td>
                                <StatusBadge status={r.status} />
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}

                {/* Slot Checks - always show */}
                {stateResult.slotResults && stateResult.slotResults.length > 0 && (
                  <div className={styles.subsection}>
                    <h5>Slot Checks ({stateResult.slotResults.length})</h5>
                    <table className={styles.table}>
                      <thead>
                        <tr>
                          {verbose && <th>Slot</th>}
                          <th>Name</th>
                          <th>Expected</th>
                          <th>Actual</th>
                          <th>Status</th>
                        </tr>
                      </thead>
                      <tbody>
                        {stateResult.slotResults.map((r, i) => {
                          const comment = comments.slots.get(r.slot);
                          return (
                            <tr key={i}>
                              {verbose && (
                                <td>
                                  <code>{r.slot}</code>
                                </td>
                              )}
                              <td>
                                <span className={styles.functionCell}>
                                  {r.name}
                                  {comment && <InfoTooltip comment={comment} />}
                                </span>
                              </td>
                              <td>
                                <code>{String(r.expected)}</code>
                              </td>
                              <td>
                                <code>{String(r.actual)}</code>
                              </td>
                              <td>
                                <StatusBadge status={r.status} />
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}

                {/* Storage Paths - always show */}
                {stateResult.storagePathResults && stateResult.storagePathResults.length > 0 && (
                  <div className={styles.subsection}>
                    <h5>Storage Paths ({stateResult.storagePathResults.length})</h5>
                    <table className={styles.table}>
                      <thead>
                        <tr>
                          <th>Path</th>
                          {verbose && <th>Computed Slot</th>}
                          <th>Expected</th>
                          <th>Actual</th>
                          <th>Status</th>
                        </tr>
                      </thead>
                      <tbody>
                        {stateResult.storagePathResults.map((r, i) => {
                          const comment = comments.storagePaths.get(r.path);
                          return (
                            <tr key={i}>
                              <td>
                                <span className={styles.functionCell}>
                                  <code>{r.path}</code>
                                  {comment && <InfoTooltip comment={comment} />}
                                </span>
                              </td>
                              {verbose && (
                                <td>
                                  <code>{r.computedSlot}</code>
                                </td>
                              )}
                              <td>
                                <code>{String(r.expected)}</code>
                              </td>
                              <td>
                                <code>{String(r.actual)}</code>
                              </td>
                              <td>
                                <StatusBadge status={r.status} />
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}

                {/* Namespace Results - show only in verbose */}
                {verbose && stateResult.namespaceResults && stateResult.namespaceResults.length > 0 && (
                  <div className={styles.subsection}>
                    <h5>Namespace Checks ({stateResult.namespaceResults.length})</h5>
                    {stateResult.namespaceResults.map((ns, ni) => (
                      <div key={ni} className={styles.namespaceGroup}>
                        <p className={styles.namespaceId}>
                          <code>{ns.namespaceId}</code> (base slot: <code>{ns.baseSlot}</code>)
                        </p>
                        <table className={styles.table}>
                          <thead>
                            <tr>
                              <th>Name</th>
                              <th>Expected</th>
                              <th>Actual</th>
                              <th>Status</th>
                            </tr>
                          </thead>
                          <tbody>
                            {ns.variables.map((v, vi) => (
                              <tr key={vi}>
                                <td>{v.name}</td>
                                <td>
                                  <code>{String(v.expected)}</code>
                                </td>
                                <td>
                                  <code>{String(v.actual)}</code>
                                </td>
                                <td>
                                  <StatusBadge status={v.status} />
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </Disclosure.Panel>
        </div>
      )}
    </Disclosure>
  );
}

export function ResultsSection() {
  const { results, clearResults, options, parsedConfig } = useVerifierStore();
  const sectionRef = useRef<HTMLDivElement>(null);

  // Scroll to results when they appear
  useEffect(() => {
    if (results && sectionRef.current) {
      sectionRef.current.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }, [results]);

  if (!results) {
    return null;
  }

  const verbose = options.verbose;

  // Extract comments for each contract from the raw config content
  const getCommentsForContract = (contractName: string): ConfigComments => {
    if (!parsedConfig?.rawContent) {
      return { viewCalls: [], slots: new Map(), storagePaths: new Map() };
    }
    return extractCommentsFromConfig(contractName, parsedConfig.rawContent, parsedConfig.format);
  };

  return (
    <div ref={sectionRef}>
      <Card title="Verification Results">
        <div className={styles.summary}>
          <div className={clsx(styles.summaryItem, styles.pass)}>
            <span className={styles.summaryValue}>{results.passed}</span>
            <span className={styles.summaryLabel}>Passed</span>
          </div>
          <div className={clsx(styles.summaryItem, styles.fail)}>
            <span className={styles.summaryValue}>{results.failed}</span>
            <span className={styles.summaryLabel}>Failed</span>
          </div>
          <div className={clsx(styles.summaryItem, styles.warn)}>
            <span className={styles.summaryValue}>{results.warnings}</span>
            <span className={styles.summaryLabel}>Warnings</span>
          </div>
          <div className={clsx(styles.summaryItem, styles.skip)}>
            <span className={styles.summaryValue}>{results.skipped}</span>
            <span className={styles.summaryLabel}>Skipped</span>
          </div>
        </div>

        {verbose && <p className={styles.verboseHint}>Verbose mode: showing additional details</p>}

        <div className={styles.results}>
          {results.results.map((result, index) => (
            <ContractResultCard
              key={index}
              result={result}
              verbose={verbose}
              comments={getCommentsForContract(result.contract.name)}
            />
          ))}
        </div>

        <button onClick={clearResults} className={styles.clearButton} type="button">
          Clear results
        </button>
      </Card>
    </div>
  );
}

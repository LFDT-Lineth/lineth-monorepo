import { TASK_COMPILE_SOLIDITY_LOG_COMPILATION_ERRORS } from "hardhat/builtin-tasks/task-names";
import { subtask } from "hardhat/config";

/**
 * Solc warnings that are intentionally suppressed, scoped to a specific source file and warning code.
 *
 * Add an entry only when a warning is a deliberate, documented design choice for a specific contract.
 * The filter is intentionally narrow (file suffix + warning code) so unrelated warnings are never hidden.
 *
 * - 2018 ("Function state mutability can be restricted to view"): the `SafeExecutionConditions` guards are
 *   intentionally non-`view` so multisig UIs (e.g. Safe{Wallet}) treat them as stageable batch transactions
 *   rather than read-only calls.
 *
 * NB: `fileSuffix` is matched against the solc source path; update it if a contract is renamed or moved.
 */
const SUPPRESSED_SOLC_WARNINGS: { errorCode: string; fileSuffix: string }[] = [
  { errorCode: "2018", fileSuffix: "operational/SafeExecutionConditions.sol" },
];

interface SolcCompilationError {
  severity?: string;
  errorCode?: string | number;
  sourceLocation?: { file?: string };
}

interface SolcCompilationOutput {
  errors?: SolcCompilationError[];
}

/**
 * Overrides the Solidity compilation error logging subtask to drop the warnings listed in
 * {@link SUPPRESSED_SOLC_WARNINGS}. Every other diagnostic (including warnings from other files) is forwarded
 * unchanged, and errors are never filtered.
 */
subtask(
  TASK_COMPILE_SOLIDITY_LOG_COMPILATION_ERRORS,
  async (taskArgs: { output?: SolcCompilationOutput; quiet: boolean }, _hre, runSuper) => {
    const { output } = taskArgs;
    if (!output?.errors?.length) {
      return runSuper(taskArgs);
    }

    const filteredErrors = output.errors.filter(
      (error) =>
        !SUPPRESSED_SOLC_WARNINGS.some(
          (suppressed) =>
            error.severity === "warning" &&
            String(error.errorCode) === suppressed.errorCode &&
            (error.sourceLocation?.file ?? "").endsWith(suppressed.fileSuffix),
        ),
    );

    return runSuper({ ...taskArgs, output: { ...output, errors: filteredErrors } });
  },
);

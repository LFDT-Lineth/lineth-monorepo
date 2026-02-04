import { PendingPartialWithdrawal } from "@consensys/linea-shared-utils";
import {
  AggregatedPendingWithdrawal,
  ExitedValidator,
  ExitingValidator,
  ValidatorBalance,
  ValidatorBalanceWithPendingWithdrawal,
} from "../entities/Validator.js";

export interface IValidatorDataClient {
  getActiveValidators(): Promise<ValidatorBalance[] | undefined>;
  getExitingValidators(): Promise<ExitingValidator[] | undefined>;
  getExitedValidators(): Promise<ExitedValidator[] | undefined>;
  getValidatorsForWithdrawalRequestsAscending(): Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined>;
  joinValidatorsWithPendingWithdrawals(
    allValidators: ValidatorBalance[] | undefined,
    pendingWithdrawalsQueue: PendingPartialWithdrawal[] | undefined,
  ): ValidatorBalanceWithPendingWithdrawal[] | undefined;
  getFilteredAndAggregatedPendingWithdrawals(
    allValidators: ValidatorBalance[] | undefined,
    pendingWithdrawalsQueue: PendingPartialWithdrawal[] | undefined,
  ): AggregatedPendingWithdrawal[] | undefined;
  getTotalPendingPartialWithdrawalsWei(validatorList: ValidatorBalanceWithPendingWithdrawal[]): bigint;
  getTotalValidatorBalanceGwei(validators: ValidatorBalance[] | undefined): bigint | undefined;
  getTotalBalanceOfExitingValidators(validators: ExitingValidator[] | undefined): bigint | undefined;
  getTotalBalanceOfExitedValidators(validators: ExitedValidator[] | undefined): bigint | undefined;
}

import { ValidatorBalance, ValidatorBalanceWithPendingWithdrawal } from "../entities/ValidatorBalance.js";

export interface IValidatorDataClient {
  getActiveValidators(): Promise<ValidatorBalance[] | undefined>;
  getActiveValidatorsWithPendingWithdrawals(): Promise<ValidatorBalanceWithPendingWithdrawal[] | undefined>;
  getTotalPendingPartialWithdrawalsWei(validatorList: ValidatorBalanceWithPendingWithdrawal[]): bigint;
}

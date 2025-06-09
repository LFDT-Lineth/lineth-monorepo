# L1 Dynamic gas pricing mechainsm


## Objective

The purpose of this document is to illustrate the idea behind the L1 dynamic gas pricing mechanism, it's corresponding coordinator configurations, and the steps to enable it.

## Motivation
The goal is to reduce L1 costs by optimizing the gas price at which we send transactions (blob submission, finalization) to L1 whilst being within our SLA for finalization/withdrawals.

## Scope

We propose a mechanism that updates the L1 gas price every hour based on the time elapsed since the first L2 block in each aggregation, and the time of day/week to account for when we expect L1 prices to be lower/higher.

This would help reduce the L1 costs given that the L1 data submission transaction accounts for the major part of our total L1 cost overhead. There is now no artificial delay for submission, giving a longer window of flexibility to post at an optimal L1 gas price and be within the SLA time.

### Formula

The formula to determine the current L1 gas price `threshold` is:

`threshold = p10GasPricePastWeek (1 + adjustmentConstant * TDM * (elapsedTimeSinceAggregationFirstBlock/ SLA)^2 )`

Where:

The `p10GasPricePastWeek` monitors the 10th percentile of the L1 gas price across the last week. This requires a dB to help keep track of the latest price trends by storing the fee history data

The `elapsedTimeSinceAggregationFirstBlock` is calculated by taking the difference between the current timestamp and the timestamp of the first L2 block which is to be finalized on L1 by the current aggregation

`SLA` = 16 hours (configurable). Includes any legal delays

There are global caps for maxFeePerGas and maxPriorityFeePerGas (i.e. set by `l1.max-fee-per-gas-cap` and `l1.max-priority-fee-per-gas-cap` respectively from coordinator configs) for blob submission and the same set of max caps for finalization are 2x the max caps for blob submission. This should be configurable and the L1 gas price can not go above this to protect from spikes or if the formula goes out of control

The `adjustmentConstant` is used to adjust the exponential increase in gas price. The higher the constant the faster the gas price increases as the `elapsedTimeSinceAggregationFirstBlock` increases = 25 (configurable)

`TDM` stands for "Time of Day Mutliplier" and has a range of values between 0.25 - 1.75. When we expect the L1 gas price to be low, for example on a Saturday night, the `TDM` will be at it's highest because we want to increase the rate at which we increase the threshold to try and settle within this time window. The value will be at it's lowest during peak L1 gas price times, for example, on a Tuesday afternoon, because we know it is likely to be expensive during this window. (Here's the [toml file in repo](https://github.com/Consensys/linea-monorepo/blob/01925cd1e519b44f94f450964d271ae05735db22/config/common/gas-price-cap-time-of-day-multipliers.toml#L16))


The implementation of the above equation can be found [here](https://github.com/Consensys/linea-monorepo/blob/main/coordinator/ethereum/gas-pricing/dynamic-cap/src/main/kotlin/net/consensys/linea/ethereum/gaspricing/dynamiccap/GasPriceCapCalculatorImpl.kt)

### Here's the final version of the formulas:
- *Assuming the "percentile" is set as 10th*

#### Aggregation Finalization
[implementation](https://github.com/Consensys/linea-monorepo/blob/main/coordinator/ethereum/gas-pricing/dynamic-cap/src/main/kotlin/net/consensys/linea/ethereum/gaspricing/dynamiccap/GasPriceCapProviderForFinalization.kt)
``` 
baseFeeCap = p10(baseFeePastWeek) * (1 + adjustmentConstant * TDM * (elapsedTimeSinceAggregationFirstBlock / SLA)^2 )
priorityFeeCap = average(p10rewardsPastWeek) * (1 + adjustmentConstant * TDM * (elapsedTimeSinceAggregationFirstBlock / SLA)^2 )

maxPriorityFeePerGas = min(priorityFeeCap, priority-fee-per-gas-upper-bound-for-aggregation)
maxFeePerGas = min(baseFeeCap + maxPriorityFeePerGas, max-fee-per-gas-cap-for-aggregation)
```

#### Blob Submission 
[implementation](https://github.com/Consensys/linea-monorepo/blob/main/coordinator/ethereum/gas-pricing/dynamic-cap/src/main/kotlin/net/consensys/linea/ethereum/gaspricing/dynamiccap/GasPriceCapProviderForDataSubmission.kt)
```
baseFeeCap = p10(baseFeePastWeek) * (1 + adjustmentConstant * TDM * (elapsedTimeSinceAggregationFirstBlock / SLA)^2 )
priorityFeeCap = average(p10rewardsPastWeek) * (1 + adjustmentConstant * TDM * (elapsedTimeSinceAggregationFirstBlock / SLA)^2 )
blobBaseFeeCap = p10(blobBaseFeePastWeek) * (1 + blobAdjustmentConstant * blobTDM * (elapsedTimeSinceAggregationFirstBlock / SLA)^2 )

maxPriorityFeePerGas = min(priorityFeeCap, priority-fee-per-gas-upper-bound)
maxFeePerGas = min(baseFeeCap + maxPriorityFeePerGas, max-fee-per-gas-cap)
maxFeePerBlobGas= min(blobBaseFeeCap, max-fee-per-blob-gas-cap)
```

Also note that, we will submit blob-carrying txns on L1 only if the dynamic gas prices are no less than the current base gas prices on L1, in order to cope with "100% replacement penalty" for blob-carrying txns.

## Related coordinator configurations

```jsx
[l1-submission.dynamic-gas-price-cap]
disabled=false //if false, dynamic gas price cap will be enabled, and if true, gas price cap will behave as static just like before

[l1-submission.dynamic-gas-price-cap.gas-price-cap-calculation]
adjustment-constant=25 //used in the dynamic gas price cap equation
blob-adjustment-constant=25 //used in the dynamic gas price cap equation
finalization-target-max-delay="PT32H" //used in the dynamic gas price cap equation which is derived by our service level agreement that a L2 block should be finalized on L1 within 32 hours
base-fee-per-gas-percentile-window="P7D" //used by fee history repository to calculate the p10(baseFeePastWeek)
base-fee-per-gas-percentile-window-leeway="PT10M" //used by gas price cap provider to determine whether the currently cached fee history data is enough for gas price cap calculation, e.g. this means 50400 - 50 = 50350 blocks of fee history data from the past 7 days would be sufficient to proceed the calculation; otherwise, will fallback to static gas pricing mechanism
base-fee-per-gas-percentile=10.0 //used by fee history repository to calculate the p10(baseFeePastWeek)
gas-price-caps-check-coefficient=0.9 //used for multiplying the dynamic gas price caps and check if they are higher than the current base gas fees on L1, this normally should be less than 1.0
historic-base-fee-per-blob-gas-lower-bound=100000000 //used as lower bound for p10(baseFeePerBlobGasPastWeek) from the dynamic gas price cap equation
historic-avg-reward-constant=100000000 //used as the replacement of the average of p10(rewardPastWeek) from the dynamic gas price cap equation

[l1-submission.dynamic-gas-price-cap.fee-history-fetcher]
fetch-interval="PT1S" //periodic interval for the fee history caching service to fetch L1 fee history data
l1-endpoint="http://l1-el-node:8545" //optional endpoint used by the fee history fetcher to make eth_feeHistory calls, if not given, the one defined in l1.eth-fee-history-endpoint or else l1.rpc-endpoint will be used
max-block-count=1000 //MAX block count value for fee history fetcher to be used in eth_feeHistory call (1000 is the most RPC call can support)
reward-percentiles=[10,20,30,40,50,60,70,80,90,100] //reward percentiles for fee history fetcher to be used in eth_feeHistory call
num-of-blocks-before-latest=4 // optional number of blocks (default as 4) to avoid requesting fee history of the head block from nodes that were not catching up with the chain head yet
storage-period="P10D" //MAX storage period for fee history repository to prune db, e.g. 10 days means the db would not contain records with block number < ("latest L1 block number" - 72000)
```

Related configs for max caps of L1 gas fees (under the `[l1-submission.blob]` and `[l1-submission.aggregation]` sections):

- `max-fee-per-gas-cap` if we want to change the max cap of `maxFeePerGas`
- `max-fee-per-blob-gas-cap` if we want to change the max cap of `maxFeePerBlobGas` (`[l1-submission.blob] only`)
- `max-priority-fee-per-gas-cap` if we want to change the max cap of `maxPriorityFeePerGas`

## Steps to enable

1. We can enable the L1 dynamic gas pricing mechanism by setting the following config, given that the other parameters as mentioned above have been properly set:
    - `[l1-submission.dynamic-gas-price-cap] disabled=false`
3. Start/Restart the coordinator

## Steps to verify

Once the L1 dynamic gas pricing mechanism is enabled, it might take some time (few minutes) for the database to store enough fee history data, i.e. it needs to retrieve at least the last 7 days of fee history data from the current L1 block.

The effects would be as following:

- `maxFeePerGas`, `maxPriorityFePerGas`, and `maxFeePerBlobGas` for a blob submission transactions will be increased gradually for each re-submission on L1 and their max capped values would be the value we set for `l1-submission.blob.max-fee-per-gas-cap` , `l1-submission.blob.maxPriorityFeePerGas` , and `l1-submission.blob.max-fee-per-blob-gas-cap` respectively
- `maxFeePerGas` and `maxPriorityFePerGas`for a finalization transactions will be increased gradually for each re-submission on L1 and their max capped values would be the value we set for `l1-submission.aggregation.max-fee-per-gas-cap`  and `l1-submission.aggregation.maxPriorityFeePerGas` respectively
- The increasing rate of these max gas fees are defined by the equations mentioned above

**Coordinator Logs**

- If we see the following in the coordinator logs, then it indicates the L1 dynamic gas pricing mechanism is effective:
    - `finalizeBlocks gas price caps: blobCarrying=false maxPriorityFeePerGas=0.001129593 GWei, dynamicMaxPriorityFeePerGas=3.823859529 GWei, maxFeePerGas=0.038442435 GWei, dynamicMaxFeePerGas=3.824274318 GWei`
    - `submitBlobs gas price caps: blobCarrying=true maxPriorityFeePerGas=2.0 GWei, dynamicMaxPriorityFeePerGas=3.812889071 GWei, maxFeePerGas=2.035904418 GWei, dynamicMaxFeePerGas=3.81330267 GWei, maxFeePerBlobGas=5000.0 GWei, dynamicMaxFeePerBlobGas=3.251486911 GWei` 
- If we see  `Not enough fee history data for gas price cap update`, then it means the fee history data in database is not sufficient, and the gas pricing mechanism will be fallback to the static gas pricing mechanism, i.e. using the global caps (`l1.max-fee-per-gas-cap`, `l1.max-fee-per-blob-gas-cap`, `l1.max-priority-fee-per-gas-cap`) directly
- If we see the followings in the log on `eth_call`:
    - `eth_call for blob submission failed: blob=[13513839..13514420]582 errorMessage=maxFeePerGas less than block base fee: address 0x47C63d1E391FcB3dCdC40C4d7fA58ADb172f8c38, maxFeePerGas: 20033351340, baseFee: 21544625157 (supplied gas 1000000)`
    - `eth_call for blob submission failed: blob=[18690037..18690464]428 errorMessage=maxFeePerBlobGas less than block blob gas fee: address 0x46d2F319fd42165D4318F099E143dEA8124E9E3e maxFeePerBlobGas: 4350230936, blobBaseFee: 4474834655 (supplied gas 1100000)`
    
    It means the blob-carrying transaction that is about to submit doesn’t have maxFeePerGas or maxFeePerBlobGas higher than the current base gas fees on L1, and the coordinator will not submit it on L1, this can be quite common to see in log when dynamic gas pricing is enabled
- If we see the following in the logs:
    - `replacement transaction underpriced: new tx gas tip cap 2500000000 <= 2000000000 queued + 100% replacement penalty`
    - `replacement transaction underpriced: new tx gas fee cap 40733292344 <= 32144797253 queued + 100% replacement penalty`
    - `replacement transaction underpriced: new tx blob gas fee cap 485826427361 <= 243239997227 queued + 100% replacement penalty`
    
    It means the pending blob-carrying transaction was not able to be replaced with the current dynamic gas prices due to the “100% replacement penalty” for blob-carrying transactions (see [here](https://github.com/colinlyguo/EIP-4844-dev-usage?tab=readme-ov-file#:~:text=Due%20to%20blob%20pool%27s%20constraints)), which in most case is normal, as the dynamic gas prices would eventually climb up to be doubled as the pending ones within a reasonable timeframe (e.g. long before the 32 hours SLA)

**Coordinator Metrics**

The following Prometheus metrics are exposed by Coordinator for L1 dynamic gas pricing:

- linea_gas_price_cap_l1_blobsubmission_maxfeepergascap
- linea_gas_price_cap_l1_blobsubmission_maxfeeperblobgascap
- linea_gas_price_cap_l1_blobsubmission_maxpriorityfeepergascap
- linea_gas_price_cap_l1_finalization_maxfeepergascap
- linea_gas_price_cap_l1_finalization_maxpriorityfeepergascap

They are all as Gauge metrics and will only be updated upon L1 tx submissions, therefore please be caution that if they were being flat or zero for a long period of time, then it might indicate that L1 dynamic gas pricing was not being used (e.g. due to fee history retrieval errors) or there were no transactions to submit on L1 (e.g. due to L2 block retrieval errors or provers not able to generate new compression/aggregation proofs)
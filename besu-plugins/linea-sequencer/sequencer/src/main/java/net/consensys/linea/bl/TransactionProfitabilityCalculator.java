/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.bl;

import java.math.BigDecimal;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.utils.Compressor;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.slf4j.spi.LoggingEventBuilder;

/**
 * This class implements the profitability formula, and it is used both to check if a tx is
 * profitable and to give an estimation of the profitable priorityFeePerGas for a given tx. The
 * profitability depends on the context, so it could mean that it is priced enough to have a chance:
 * to be accepted in the txpool and to be a candidate for new block creation, it is also used to
 * give an estimated priorityFeePerGas in response to a linea_estimateGas call. Each context has it
 * own minMargin configuration, so that is possible to accept in the txpool txs, that are not yet
 * profitable for block inclusion, but could be in future if the gas price decrease and likewise, it
 * is possible to return an estimated priorityFeePerGas that has a profitability buffer to address
 * small fluctuations in the gas market.
 */
@Slf4j
public class TransactionProfitabilityCalculator {
  private final LineaProfitabilityConfiguration profitabilityConf;

  public TransactionProfitabilityCalculator(
      final LineaProfitabilityConfiguration profitabilityConf) {
    this.profitabilityConf = profitabilityConf;
  }

  /**
   * Calculate the estimation of priorityFeePerGas that is considered profitable for the given tx,
   * according to the current pricing config and the minMargin.
   *
   * @param transaction the tx we want to get the estimated priorityFeePerGas for
   * @param minMargin the min margin to use for this calculation
   * @param gas the gas to use for this calculation, could be the gasUsed of the tx, if it has been
   *     processed/simulated, otherwise the gasLimit of the tx
   * @param minGasPriceWei the current minGasPrice, only used in place of the variable cost from the
   *     config, in case the extra data pricing is disabled
   * @return the estimation of priorityFeePerGas that is considered profitable for the given tx
   */
  public Wei profitablePriorityFeePerGas(
      final Transaction transaction,
      final double minMargin,
      final long gas,
      final Wei minGasPriceWei) {
    final int compressedTxSize = getCompressedTxSize(transaction);

    final long variableCostWei =
        profitabilityConf.extraDataPricingEnabled()
            ? profitabilityConf.variableCostWei()
            : minGasPriceWei.toLong();

    final var profitAt =
        minMargin * (variableCostWei * compressedTxSize / gas + profitabilityConf.fixedCostWei());

    final var profitAtWei = Wei.ofNumber(BigDecimal.valueOf(profitAt).toBigInteger());

    log.atDebug()
        .setMessage(
            "Estimated profitable priorityFeePerGas: {}; minMargin={}, fixedCostWei={}, "
                + "variableCostWei={}, gas={}, txSize={}, compressedTxSize={}")
        .addArgument(profitAtWei::toHumanReadableString)
        .addArgument(minMargin)
        .addArgument(profitabilityConf.fixedCostWei())
        .addArgument(variableCostWei)
        .addArgument(gas)
        .addArgument(transaction::getSizeForBlockInclusion)
        .addArgument(compressedTxSize)
        .log();

    return profitAtWei;
  }

  /**
   * Checks if then given gas price is considered profitable for the given tx, according to the
   * current pricing config, the minMargin and gas used, or gasLimit of the tx.
   *
   * @param context a string to name the context in which it is called, used for logs
   * @param transaction the tx we want to check if profitable
   * @param minMargin the min margin to use for this check
   * @param payingGasPrice the gas price the tx is willing to pay
   * @param gas the gas to use for this check, could be the gasUsed of the tx, if it has been
   *     processed/simulated, otherwise the gasLimit of the tx
   * @param minGasPriceWei the current minGasPrice, only used in place of the variable cost from the
   *     config, in case the extra data pricing is disabled
   * @return true if the tx is priced enough to be profitable, false otherwise
   */
  public boolean isProfitable(
      final String context,
      final Transaction transaction,
      final double minMargin,
      final Wei baseFee,
      final Wei payingGasPrice,
      final long gas,
      final Wei minGasPriceWei) {

    final Wei profitablePriorityFee =
        profitablePriorityFeePerGas(transaction, minMargin, gas, minGasPriceWei);

    return isProfitable(
        context,
        profitablePriorityFee,
        transaction,
        minMargin,
        baseFee,
        payingGasPrice,
        gas,
        minGasPriceWei);
  }

  public boolean isProfitable(
      final String context,
      final Wei profitablePriorityFee,
      final Transaction transaction,
      final double minMargin,
      final Wei baseFee,
      final Wei payingGasPrice,
      final long gas,
      final Wei minGasPriceWei) {

    final Wei profitableGasPrice = baseFee.add(profitablePriorityFee);

    if (payingGasPrice.lessThan(profitableGasPrice)) {
      log(
          log.atDebug(),
          context,
          transaction,
          minMargin,
          payingGasPrice,
          baseFee,
          profitablePriorityFee,
          profitableGasPrice,
          gas,
          minGasPriceWei);
      return false;
    }

    log(
        log.atTrace(),
        context,
        transaction,
        minMargin,
        payingGasPrice,
        baseFee,
        profitablePriorityFee,
        profitableGasPrice,
        gas,
        minGasPriceWei);
    return true;
  }

  /**
   * This method calculates the compressed size of a tx using the native lib
   *
   * @param transaction the tx
   * @return the compressed size
   */
  private int getCompressedTxSize(final Transaction transaction) {
    final byte[] bytes = transaction.encoded().toArrayUnsafe();
    return Compressor.instance.compressedSize(bytes);
  }

  private void log(
      final LoggingEventBuilder leb,
      final String context,
      final Transaction transaction,
      final double minMargin,
      final Wei payingGasPrice,
      final Wei baseFee,
      final Wei profitablePriorityFee,
      final Wei profitableGasPrice,
      final long gasUsed,
      final Wei minGasPriceWei) {

    leb.setMessage(
            "Context {}. Transaction {} has a margin of {}, minMargin={}, payingGasPrice={},"
                + " profitableGasPrice={}, baseFee={}, profitablePriorityFee={}, fixedCostWei={}, variableCostWei={}, "
                + " gasUsed={}")
        .addArgument(context)
        .addArgument(transaction::getHash)
        .addArgument(
            () ->
                payingGasPrice.toBigInteger().doubleValue()
                    / profitablePriorityFee.toBigInteger().doubleValue())
        .addArgument(minMargin)
        .addArgument(payingGasPrice::toHumanReadableString)
        .addArgument(profitableGasPrice::toHumanReadableString)
        .addArgument(baseFee::toHumanReadableString)
        .addArgument(profitablePriorityFee::toHumanReadableString)
        .addArgument(profitabilityConf::fixedCostWei)
        .addArgument(
            () ->
                profitabilityConf.extraDataPricingEnabled()
                    ? profitabilityConf.variableCostWei()
                    : minGasPriceWei.toLong())
        .addArgument(gasUsed)
        .log();
  }
}

/*
 * Copyright ConsenSys Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package net.consensys.linea.zktracer.module.tables;

import static net.consensys.linea.zktracer.Trace.OOB_INST_BLS_G1_MSM;
import static net.consensys.linea.zktracer.Trace.OOB_INST_BLS_G2_MSM;
import static net.consensys.linea.zktracer.Trace.PRC_BLS_G1_MSM_MAX_DISCOUNT;
import static net.consensys.linea.zktracer.Trace.PRC_BLS_G2_MSM_MAX_DISCOUNT;
import static net.consensys.linea.zktracer.module.ModuleName.BLS_REFERENCE_TABLE;

import com.google.common.base.Preconditions;
import java.util.List;
import java.util.Map;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.module.ModuleName;

public class BlsRt implements Module {
  @Override
  public ModuleName moduleKey() {
    return BLS_REFERENCE_TABLE;
  }

  @Override
  public void popTransactionBundle() {}

  @Override
  public void commitTransactionBundle() {}

  @Override
  public int lineCount() {
    return 128 * 2;
  }

  @Override
  public int spillage(Trace trace) {
    return trace.blsreftable().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.blsreftable().headers(lineCount());
  }

  // TODO: double check
  public static final Map<Integer, Integer> G1_MSM_DISCOUNTS =
      Map.<Integer, Integer>ofEntries(
          Map.entry(1, 1000),
          Map.entry(2, 949),
          Map.entry(3, 848),
          Map.entry(4, 797),
          Map.entry(5, 764),
          Map.entry(6, 750),
          Map.entry(7, 738),
          Map.entry(8, 728),
          Map.entry(9, 719),
          Map.entry(10, 712),
          Map.entry(11, 705),
          Map.entry(12, 698),
          Map.entry(13, 692),
          Map.entry(14, 687),
          Map.entry(15, 682),
          Map.entry(16, 677),
          Map.entry(17, 673),
          Map.entry(18, 669),
          Map.entry(19, 665),
          Map.entry(20, 661),
          Map.entry(21, 658),
          Map.entry(22, 654),
          Map.entry(23, 651),
          Map.entry(24, 648),
          Map.entry(25, 645),
          Map.entry(26, 642),
          Map.entry(27, 640),
          Map.entry(28, 637),
          Map.entry(29, 635),
          Map.entry(30, 632),
          Map.entry(31, 630),
          Map.entry(32, 627),
          Map.entry(33, 625),
          Map.entry(34, 623),
          Map.entry(35, 621),
          Map.entry(36, 619),
          Map.entry(37, 617),
          Map.entry(38, 615),
          Map.entry(39, 613),
          Map.entry(40, 611),
          Map.entry(41, 609),
          Map.entry(42, 608),
          Map.entry(43, 606),
          Map.entry(44, 604),
          Map.entry(45, 603),
          Map.entry(46, 601),
          Map.entry(47, 599),
          Map.entry(48, 598),
          Map.entry(49, 596),
          Map.entry(50, 595),
          Map.entry(51, 593),
          Map.entry(52, 592),
          Map.entry(53, 591),
          Map.entry(54, 589),
          Map.entry(55, 588),
          Map.entry(56, 586),
          Map.entry(57, 585),
          Map.entry(58, 584),
          Map.entry(59, 582),
          Map.entry(60, 581),
          Map.entry(61, 580),
          Map.entry(62, 579),
          Map.entry(63, 577),
          Map.entry(64, 576),
          Map.entry(65, 575),
          Map.entry(66, 574),
          Map.entry(67, 573),
          Map.entry(68, 572),
          Map.entry(69, 570),
          Map.entry(70, 569),
          Map.entry(71, 568),
          Map.entry(72, 567),
          Map.entry(73, 566),
          Map.entry(74, 565),
          Map.entry(75, 564),
          Map.entry(76, 563),
          Map.entry(77, 562),
          Map.entry(78, 561),
          Map.entry(79, 560),
          Map.entry(80, 559),
          Map.entry(81, 558),
          Map.entry(82, 557),
          Map.entry(83, 556),
          Map.entry(84, 555),
          Map.entry(85, 554),
          Map.entry(86, 553),
          Map.entry(87, 552),
          Map.entry(88, 551),
          Map.entry(89, 550),
          Map.entry(90, 549),
          Map.entry(91, 548),
          Map.entry(92, 547),
          Map.entry(93, 547),
          Map.entry(94, 546),
          Map.entry(95, 545),
          Map.entry(96, 544),
          Map.entry(97, 543),
          Map.entry(98, 542),
          Map.entry(99, 541),
          Map.entry(100, 540),
          Map.entry(101, 540),
          Map.entry(102, 539),
          Map.entry(103, 538),
          Map.entry(104, 537),
          Map.entry(105, 536),
          Map.entry(106, 536),
          Map.entry(107, 535),
          Map.entry(108, 534),
          Map.entry(109, 533),
          Map.entry(110, 532),
          Map.entry(111, 532),
          Map.entry(112, 531),
          Map.entry(113, 530),
          Map.entry(114, 529),
          Map.entry(115, 528),
          Map.entry(116, 528),
          Map.entry(117, 527),
          Map.entry(118, 526),
          Map.entry(119, 525),
          Map.entry(120, 525),
          Map.entry(121, 524),
          Map.entry(122, 523),
          Map.entry(123, 522),
          Map.entry(124, 522),
          Map.entry(125, 521),
          Map.entry(126, 520),
          Map.entry(127, 520),
          Map.entry(128, 519));

  public static final Map<Integer, Integer> G2_MSM_DISCOUNTS =
      Map.<Integer, Integer>ofEntries(
          Map.entry(1, 1000),
          Map.entry(2, 1000),
          Map.entry(3, 923),
          Map.entry(4, 884),
          Map.entry(5, 855),
          Map.entry(6, 832),
          Map.entry(7, 812),
          Map.entry(8, 796),
          Map.entry(9, 782),
          Map.entry(10, 770),
          Map.entry(11, 759),
          Map.entry(12, 749),
          Map.entry(13, 740),
          Map.entry(14, 732),
          Map.entry(15, 724),
          Map.entry(16, 717),
          Map.entry(17, 711),
          Map.entry(18, 704),
          Map.entry(19, 699),
          Map.entry(20, 693),
          Map.entry(21, 688),
          Map.entry(22, 683),
          Map.entry(23, 679),
          Map.entry(24, 674),
          Map.entry(25, 670),
          Map.entry(26, 666),
          Map.entry(27, 663),
          Map.entry(28, 659),
          Map.entry(29, 655),
          Map.entry(30, 652),
          Map.entry(31, 649),
          Map.entry(32, 646),
          Map.entry(33, 643),
          Map.entry(34, 640),
          Map.entry(35, 637),
          Map.entry(36, 634),
          Map.entry(37, 632),
          Map.entry(38, 629),
          Map.entry(39, 627),
          Map.entry(40, 624),
          Map.entry(41, 622),
          Map.entry(42, 620),
          Map.entry(43, 618),
          Map.entry(44, 615),
          Map.entry(45, 613),
          Map.entry(46, 611),
          Map.entry(47, 609),
          Map.entry(48, 607),
          Map.entry(49, 606),
          Map.entry(50, 604),
          Map.entry(51, 602),
          Map.entry(52, 600),
          Map.entry(53, 598),
          Map.entry(54, 597),
          Map.entry(55, 595),
          Map.entry(56, 593),
          Map.entry(57, 592),
          Map.entry(58, 590),
          Map.entry(59, 589),
          Map.entry(60, 587),
          Map.entry(61, 586),
          Map.entry(62, 584),
          Map.entry(63, 583),
          Map.entry(64, 582),
          Map.entry(65, 580),
          Map.entry(66, 579),
          Map.entry(67, 578),
          Map.entry(68, 576),
          Map.entry(69, 575),
          Map.entry(70, 574),
          Map.entry(71, 573),
          Map.entry(72, 571),
          Map.entry(73, 570),
          Map.entry(74, 569),
          Map.entry(75, 568),
          Map.entry(76, 567),
          Map.entry(77, 566),
          Map.entry(78, 565),
          Map.entry(79, 563),
          Map.entry(80, 562),
          Map.entry(81, 561),
          Map.entry(82, 560),
          Map.entry(83, 559),
          Map.entry(84, 558),
          Map.entry(85, 557),
          Map.entry(86, 556),
          Map.entry(87, 555),
          Map.entry(88, 554),
          Map.entry(89, 553),
          Map.entry(90, 552),
          Map.entry(91, 552),
          Map.entry(92, 551),
          Map.entry(93, 550),
          Map.entry(94, 549),
          Map.entry(95, 548),
          Map.entry(96, 547),
          Map.entry(97, 546),
          Map.entry(98, 545),
          Map.entry(99, 545),
          Map.entry(100, 544),
          Map.entry(101, 543),
          Map.entry(102, 542),
          Map.entry(103, 541),
          Map.entry(104, 541),
          Map.entry(105, 540),
          Map.entry(106, 539),
          Map.entry(107, 538),
          Map.entry(108, 537),
          Map.entry(109, 537),
          Map.entry(110, 536),
          Map.entry(111, 535),
          Map.entry(112, 535),
          Map.entry(113, 534),
          Map.entry(114, 533),
          Map.entry(115, 532),
          Map.entry(116, 532),
          Map.entry(117, 531),
          Map.entry(118, 530),
          Map.entry(119, 530),
          Map.entry(120, 529),
          Map.entry(121, 528),
          Map.entry(122, 528),
          Map.entry(123, 527),
          Map.entry(124, 526),
          Map.entry(125, 526),
          Map.entry(126, 525),
          Map.entry(127, 524),
          Map.entry(128, 524));

  public static int getMsmDiscount(final int instruction, final int numInputs) {
    Preconditions.checkArgument(numInputs >= 1, "Number of inputs must be at least 1");
    return switch (instruction) {
      case OOB_INST_BLS_G1_MSM ->
          numInputs <= 128 ? G1_MSM_DISCOUNTS.get(numInputs) : PRC_BLS_G1_MSM_MAX_DISCOUNT;
      case OOB_INST_BLS_G2_MSM ->
          numInputs <= 128 ? G2_MSM_DISCOUNTS.get(numInputs) : PRC_BLS_G2_MSM_MAX_DISCOUNT;
      default -> throw new IllegalArgumentException("Invalid instruction: " + instruction);
    };
  }

  public void commit(Trace trace) {
    for (Map.Entry<Integer, Integer> entry : G1_MSM_DISCOUNTS.entrySet()) {
      int numInputs = entry.getKey();
      int discount = entry.getValue();
      trace
          .blsreftable()
          .prcName(OOB_INST_BLS_G1_MSM)
          .numInputs(numInputs)
          .discount(discount)
          .validateRow();
    }
    for (Map.Entry<Integer, Integer> entry : G2_MSM_DISCOUNTS.entrySet()) {
      int numInputs = entry.getKey();
      int discount = entry.getValue();
      trace
          .blsreftable()
          .prcName(OOB_INST_BLS_G2_MSM)
          .numInputs(numInputs)
          .discount(discount)
          .validateRow();
    }
  }
}

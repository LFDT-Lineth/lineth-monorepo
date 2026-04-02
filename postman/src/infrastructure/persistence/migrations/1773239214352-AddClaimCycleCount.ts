import { MigrationInterface, QueryRunner, TableColumn } from "typeorm";

export class AddClaimCycleCount1773239214352 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.addColumn(
      "message",
      new TableColumn({
        name: "claim_cycle_count",
        type: "integer",
        default: 0,
        isNullable: false,
      }),
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropColumn("message", "claim_cycle_count");
  }
}

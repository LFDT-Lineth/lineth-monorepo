import { MigrationInterface, QueryRunner, TableColumn } from "typeorm";

export class AddNewColumns1687890326970 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.addColumns("message", [
      new TableColumn({
        name: "claim_number_of_retry",
        type: "integer",
        isNullable: false,
        default: 0,
      }),
      new TableColumn({
        name: "claim_last_retried_at",
        type: "timestamp with time zone",
        isNullable: true,
      }),
      new TableColumn({
        name: "claim_gas_estimation_threshold",
        type: "numeric",
        isNullable: true,
      }),
    ]);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropColumn("message", "claim_number_of_retry");
    await queryRunner.dropColumn("message", "claim_last_retried_at");
    await queryRunner.dropColumn("message", "claim_gas_estimation_threshold");
  }
}

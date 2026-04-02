import { MigrationInterface, QueryRunner, TableColumn } from "typeorm";

export class AddSponsorshipMetrics1745569276097 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.addColumns("message", [
      new TableColumn({
        name: "is_for_sponsorship",
        type: "boolean",
        isNullable: false,
        default: false,
      }),
    ]);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropColumn("message", "is_for_sponsorship");
  }
}

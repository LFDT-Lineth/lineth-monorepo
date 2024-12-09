import { MigrationInterface, QueryRunner, TableColumn } from "typeorm";

export class AddCompressedTxSizeColumn1718026260629 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.addColumn(
      "message",
      new TableColumn({
        name: "compressed_transaction_size",
        type: "numeric",
        isNullable: true,
      }),
    );
    await queryRunner.query(`ALTER TYPE "statusEnum" ADD VALUE 'TRANSACTION_SIZE_COMPUTED'`);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropColumn("message", "compressed_transaction_size");
    await queryRunner.query(`ALTER TYPE "statusEnum" DROP VALUE 'TRANSACTION_SIZE_COMPUTED'`);
  }
}

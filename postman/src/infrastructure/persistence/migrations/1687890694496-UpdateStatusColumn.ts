import { MigrationInterface, QueryRunner } from "typeorm";

export class UpdateStatusColumn1687890694496 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`ALTER TYPE "statusEnum" ADD VALUE 'ZERO_FEE'`);
    await queryRunner.query(`ALTER TYPE "statusEnum" ADD VALUE 'FEE_UNDERPRICED'`);
    await queryRunner.query(`ALTER TYPE "statusEnum" ADD VALUE 'EXCLUDED'`);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`ALTER TYPE "statusEnum" DROP VALUE 'ZERO_FEE'`);
    await queryRunner.query(`ALTER TYPE "statusEnum" DROP VALUE 'FEE_UNDERPRICED'`);
    await queryRunner.query(`ALTER TYPE "statusEnum" DROP VALUE 'EXCLUDED'`);
  }
}

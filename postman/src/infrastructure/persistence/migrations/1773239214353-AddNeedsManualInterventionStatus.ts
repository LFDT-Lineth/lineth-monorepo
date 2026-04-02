import { MigrationInterface, QueryRunner } from "typeorm";

export class AddNeedsManualInterventionStatus1773239214353 implements MigrationInterface {
  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`ALTER TYPE "statusEnum" ADD VALUE 'NEEDS_MANUAL_INTERVENTION'`);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`ALTER TYPE "statusEnum" DROP VALUE 'NEEDS_MANUAL_INTERVENTION'`);
  }
}

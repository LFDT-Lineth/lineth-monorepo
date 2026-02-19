-- CreateEnum
CREATE TYPE "ProposalSource" AS ENUM ('DISCOURSE', 'SNAPSHOT', 'LDO_VOTING_CONTRACT', 'STETH_VOTING_CONTRACT');

-- CreateEnum
CREATE TYPE "ProposalState" AS ENUM ('NEW', 'ANALYZED', 'ANALYSIS_FAILED', 'NOTIFY_FAILED', 'NOT_NOTIFIED', 'NOTIFIED');

-- CreateTable
CREATE TABLE "proposals" (
    "id" TEXT NOT NULL,
    "source" "ProposalSource" NOT NULL,
    "source_id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "url" TEXT NOT NULL,
    "title" TEXT NOT NULL,
    "author" TEXT,
    "source_created_at" TIMESTAMP(3) NOT NULL,
    "raw_proposal_text" TEXT NOT NULL,
    "source_metadata" JSONB,
    "state" "ProposalState" NOT NULL DEFAULT 'NEW',
    "state_updated_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "analysis_attempt_count" INTEGER NOT NULL DEFAULT 0,
    "llm_model" TEXT,
    "risk_threshold" INTEGER,
    "assessment_prompt_version" TEXT,
    "analyzed_at" TIMESTAMP(3),
    "assessment_json" JSONB,
    "risk_score" INTEGER,
    "notify_attempt_count" INTEGER NOT NULL DEFAULT 0,
    "notified_at" TIMESTAMP(3),

    CONSTRAINT "proposals_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "proposals_state_state_updated_at_idx" ON "proposals"("state", "state_updated_at");

-- CreateIndex
CREATE INDEX "proposals_source_source_created_at_idx" ON "proposals"("source", "source_created_at");

-- CreateIndex
CREATE UNIQUE INDEX "proposals_source_source_id_key" ON "proposals"("source", "source_id");

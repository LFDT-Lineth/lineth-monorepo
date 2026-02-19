# Database Migration Strategy

Uses Prisma Migrate with tracked migration history stored in `prisma/migrations/`.

## Development Workflow

1. Edit `prisma/schema.prisma`
2. Generate migration: `make test-db-migrate` (or manually: `DATABASE_URL=... pnpm db:migrate --name describe_change`)
3. Review generated SQL in `prisma/migrations/<timestamp>_describe_change/migration.sql`
4. Commit the `prisma/` directory with your change

Note: Prisma 7 requires `db:generate` before `db:migrate` - the Makefile handles this automatically.

## Commands

| Command | Description |
|---------|-------------|
| `make test-db` | Start test DB container and apply pending migrations |
| `make test-db-migrate` | Start test DB and generate new migration from schema diff |
| `make clean` | Stop test DB and remove volumes |
| `pnpm db:migrate --name X` | Generate new migration (development only, interactive) |
| `pnpm db:deploy` | Apply pending migrations (production-safe, no prompts) |
| `pnpm db:status` | Show which migrations have been applied |
| `pnpm db:generate` | Regenerate Prisma Client from schema |

## Production Deployment

Migrations run via init container (`prisma migrate deploy`) before the application starts. If migration fails, the pod won't start.

```yaml
initContainers:
  - name: db-migrate
    image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
    command: ["npx", "prisma", "migrate", "deploy"]
    env:
      - name: DATABASE_URL
        valueFrom:
          secretKeyRef:
            name: {{ .Values.database.secretName }}
            key: {{ .Values.database.secretKey }}
```

Verify with `kubectl logs <pod-name> -c db-migrate` or `pnpm db:status` with direct DB access.

## Troubleshooting

**Schema drift:** Run `pnpm db:status` to check. In development, `prisma migrate reset` will drop and recreate from scratch.

**Reverting a migration:** Prisma has no automatic rollback. Create a new migration that undoes the changes, or restore from backup.

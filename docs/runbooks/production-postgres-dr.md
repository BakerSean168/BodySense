# Production PostgreSQL Off-host DR Runbook

## Scope

This runbook covers `BS-PROD-012`: off-host PostgreSQL durability and restore evidence for Alibaba Cloud production.

> **Current production state (2026-09-04):** the DR manager is implemented but intentionally parked. `DR_ENABLED=false`; its systemd unit files may be installed for bootstrap readiness, but backup/status/restore timers must remain disabled until the private OSS target and ECS RAM Role are provisioned and an operator explicitly activates the plan. The older `bodysense-offhost-*` AccessKey scheduler is retired and must not be re-enabled as a substitute.
It does **not** cover durable upload/blob storage (`BS-PROD-013`).

The production contract is:

```text
PostgreSQL on ECS
→ custom-format pg_dump
→ local archive validation
→ immutable OSS object upload
→ SHA-256 + size + metadata verification
→ manifest commit marker
→ freshness monitor
→ isolated restore database
→ migration-state equality
→ domain-validator
→ disposable database deletion
```

Application rollout may use the same gate. When `DR_ENABLED=true`, `production-deploy-watch.sh` performs a verified off-host backup after syncing the candidate runtime bundle but **before** changing LiteLLM/AI/API/Web or applying database migrations.

## Alibaba Cloud one-time provisioning

Production must use an ECS RAM role. Long-lived AccessKeys are rejected by the DR manager when `APP_ENV=production`.

Create a private OSS bucket in `cn-hangzhou`, preferably using the redundancy class appropriate for the production RPO/RTO. Keep public access disabled and require HTTPS. The runtime prefix is:

```text
bodysense/production/postgres/
```

Create a custom RAM policy for the ECS role. The runtime does not need delete permissions:

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "oss:ListObjects",
      "Resource": "acs:oss:*:*:<BUCKET>",
      "Condition": {
        "StringLike": {
          "oss:Prefix": [
            "bodysense/production/postgres/*"
          ]
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "oss:GetObject",
        "oss:PutObject"
      ],
      "Resource": "acs:oss:*:*:<BUCKET>/bodysense/production/postgres/*"
    }
  ]
}
```

Attach that role to the BodySense ECS instance. The SDK retrieves and refreshes short-lived STS credentials from ECS metadata. The application runtime does not receive OSS credentials; only the one-shot `dr` Compose profile does.

Configure an OSS lifecycle rule as a bucket-administration action, not with the runtime role. Recommended starting policy:

- scope: `bodysense/production/postgres/`
- expire completed backup objects after 35 days
- abort incomplete multipart uploads after 3 days
- do not grant `oss:DeleteObject` to the ECS runtime role

Adjust retention upward if product/legal requirements need a longer recovery window.

## Production host configuration

Append host-specific values to `/opt/bodysense/.env.production.local`:

```dotenv
DR_ENABLED=true
DR_OSS_BUCKET=<PRIVATE_BUCKET>
DR_OSS_ECS_RAM_ROLE=<ECS_RAM_ROLE_NAME>
```

The tracked public defaults already provide:

```dotenv
DR_OBJECT_STORE_DRIVER=oss
DR_OSS_REGION=cn-hangzhou
DR_OSS_USE_INTERNAL_ENDPOINT=true
DR_OSS_CREDENTIAL_TYPE=ecs_ram_role
DR_OSS_PREFIX=bodysense/production/postgres
DR_OSS_SERVER_SIDE_ENCRYPTION=AES256
DR_MAX_BACKUP_AGE_HOURS=30
```

Then install/refresh timers:

```bash
/opt/bodysense/scripts/install-production-dr.sh
```

## Operator commands

```bash
/opt/bodysense/scripts/production-postgres-dr.sh backup
/opt/bodysense/scripts/production-postgres-dr.sh status
/opt/bodysense/scripts/production-postgres-dr.sh restore-drill
```

Successful results are also atomically written under `/opt/bodysense/dr-state/`.

Systemd schedule:

- backup: daily at 03:20 + randomized delay
- freshness status: every 6 hours
- restore drill: weekly Sunday 04:30 + randomized delay

Inspect with:

```bash
systemctl list-timers 'bodysense-postgres-dr-*' --all
journalctl -u bodysense-postgres-dr-backup.service -n 100
journalctl -u bodysense-postgres-dr-status.service -n 100
journalctl -u bodysense-postgres-dr-restore.service -n 100
```

`status` fails if the newest committed manifest is older than `DR_MAX_BACKUP_AGE_HOURS`, if the dump/checksum/manifest is missing, or if remote dump size/SHA metadata no longer matches the manifest.

## Backup identity and commit semantics

Each backup is stored under a revision- and timestamp-scoped immutable key:

```text
bodysense/production/postgres/YYYY/MM/DD/<timestamp>-<revision12>/
  backup.dump
  backup.dump.sha256
  manifest.json
```

`manifest.json` is uploaded **last**. `status` and restore drill only consider paths with a valid manifest, so interrupted uploads never become the latest valid backup.

The dump uses OSS SDK V2 `Uploader.UploadFile`, allowing multipart uploads for databases beyond the simple `PutObject` size limit. Object overwrite is forbidden.

## Release gate evidence

Before declaring `BS-PROD-012` complete, preserve evidence for all of the following:

1. ECS metadata returns the intended RAM role.
2. `backup` succeeds against the private OSS bucket.
3. `status` returns `remote_verified=true` and a fresh age.
4. `restore-drill` returns:
   - exact migration-state equality,
   - `domain_semantics=PASS`,
   - `dropped=true`.
5. No `bodysense_dr_*` database remains afterward.
6. The daily/weekly/freshness timers are enabled.
7. A deployment with `DR_ENABLED=true` refuses to proceed if the off-host backup gate fails.

Only after this evidence exists should the production-closeout plan mark off-host PostgreSQL DR complete.

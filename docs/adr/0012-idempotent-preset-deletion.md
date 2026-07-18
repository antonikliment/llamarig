# Idempotent Preset deletion

Deleting a missing **Preset** succeeds because deletion means ensuring that the **Preset** is absent. A missing deletion records a successful audit event but skips configured-reference checks, Router inspection or refresh, and `models.ini` writes; this keeps retries safe while avoiding misleading side effects.

-- name: RecordDelivery :one
-- Records a webhook delivery for idempotency. ON CONFLICT DO NOTHING means a
-- replayed X-GitHub-Delivery inserts nothing and returns no row (pgx.ErrNoRows),
-- which the store maps to isNew=false so the handler treats it as a no-op.
INSERT INTO webhook_deliveries (delivery_id, event)
VALUES ($1, $2)
ON CONFLICT (delivery_id) DO NOTHING
RETURNING delivery_id;

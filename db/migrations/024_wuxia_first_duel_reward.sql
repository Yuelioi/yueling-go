-- Daily reward entitlement is persisted with the operation transaction.
ALTER TABLE wuxia_daily_claims ADD COLUMN duel_rewarded boolean NOT NULL DEFAULT false;

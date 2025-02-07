CREATE TABLE shipments_3pl (
    shipment_uid TEXT PRIMARY KEY,
    start_time   TIMESTAMPTZ NOT NULL,
    retries      INTEGER NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('requested','searching','found','not_found','shipped'))
);

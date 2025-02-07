CREATE TABLE shipments (
    uid                         TEXT PRIMARY KEY,
    user_uid                    TEXT NOT NULL,
    user_addr                   TEXT NOT NULL,
    origin_point                POINT NOT NULL,
    destination_point           POINT NOT NULL,
    scheduled_delivery_min_time TIMESTAMPTZ NOT NULL,
    scheduled_delivery_max_time TIMESTAMPTZ NOT NULL,
    status                      TEXT NOT NULL CHECK (status IN ('queued','pending','requested','searching','found','not_found','shipped'))
);

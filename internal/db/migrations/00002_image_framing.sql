-- +goose Up

-- Photo framing. The stored image is never cropped; instead each recipe keeps
-- a focal point (0..1 from the left/top edge) and a zoom factor (>= 1) that
-- the web UI applies when it renders the header and the list card. The
-- defaults (centred, no zoom) match the old fixed centre crop.
ALTER TABLE recipes
    ADD COLUMN image_focus_x double precision NOT NULL DEFAULT 0.5 CHECK (image_focus_x BETWEEN 0 AND 1),
    ADD COLUMN image_focus_y double precision NOT NULL DEFAULT 0.5 CHECK (image_focus_y BETWEEN 0 AND 1),
    ADD COLUMN image_zoom    double precision NOT NULL DEFAULT 1   CHECK (image_zoom BETWEEN 1 AND 4);

-- +goose Down
ALTER TABLE recipes
    DROP COLUMN image_zoom,
    DROP COLUMN image_focus_y,
    DROP COLUMN image_focus_x;

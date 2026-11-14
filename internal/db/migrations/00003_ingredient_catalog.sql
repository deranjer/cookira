-- +goose Up
CREATE TABLE ingredient_catalog (
    id text PRIMARY KEY,
    name text NOT NULL,
    category text NOT NULL DEFAULT 'Other'
);
-- A name or alias belongs to exactly one ingredient, including concurrent writes.
CREATE TABLE ingredient_names (
    name text PRIMARY KEY,
    ingredient_id text NOT NULL REFERENCES ingredient_catalog(id) ON DELETE CASCADE
);
INSERT INTO ingredient_catalog (id, name, category) VALUES
('green-onion', 'green onion', 'Produce'),
('onion', 'onion', 'Produce'), ('garlic', 'garlic', 'Produce'),
('garlic-powder', 'garlic powder', 'Spices'),
('olive-oil', 'olive oil', 'Pantry'), ('salt', 'salt', 'Spices'),
('rice', 'rice', 'Pantry'), ('chicken-thigh', 'chicken thigh', 'Meat'),
('lime', 'lime', 'Produce'), ('cilantro', 'cilantro', 'Produce');
INSERT INTO ingredient_names SELECT name, id FROM ingredient_catalog;
INSERT INTO ingredient_names (name, ingredient_id) VALUES
('scallion', 'green-onion'), ('scallions', 'green-onion'), ('green onions', 'green-onion'),
('onions', 'onion'), ('chicken thighs', 'chicken-thigh'), ('limes', 'lime');

-- +goose Down
DROP TABLE ingredient_names;
DROP TABLE ingredient_catalog;

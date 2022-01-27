CREATE TABLE customer (
    id          SERIAL PRIMARY KEY,
    country     VARCHAR(2) NOT NULL,
    first_name  VARCHAR(50) NOT NULL,
    last_name   VARCHAR(50) NOT NULL,
    email       VARCHAR(200) NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP
);

CREATE INDEX customer_country ON customer(country);
CREATE UNIQUE INDEX customer_email ON customer(email);

CREATE TABLE customer (
    id          INT NOT NULL PRIMARY KEY,
    country     VARCHAR(2) NOT NULL,
    first_name  VARCHAR(50) NOT NULL,
    last_name   VARCHAR(50) NOT NULL,
    email       VARCHAR(200) NOT NULL,
    created_at  DATETIMEOFFSET NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIMEOFFSET
);

CREATE INDEX customer_country ON customer(country);
CREATE UNIQUE INDEX customer_email ON customer(email);

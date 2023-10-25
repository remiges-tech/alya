CREATE TABLE Employees (
                           employee_id SERIAL PRIMARY KEY, -- Unique identifier for each employee
                           name VARCHAR(100), -- Employee name
                           title VARCHAR(50), -- Job title
                           department VARCHAR(50) -- Department name
);
CREATE TABLE Vouchers (
                          voucher_id SERIAL PRIMARY KEY, -- Unique ID for each voucher
                          employee_id INT NOT NULL REFERENCES Employees(employee_id), -- Foreign Key reference to Employees table,
                          date_of_claim DATE NOT NULL, -- The claim's date; no claim without a date
                          amount DOUBLE PRECISION NOT NULL CHECK (amount > 0), -- Claimed amount; must be positive
                          description TEXT -- Description of the claim
);

CREATE TABLE schema (
                        id SERIAL PRIMARY KEY,
                        name VARCHAR(100) UNIQUE NOT NULL,
                        description VARCHAR(1000),
                        fields JSONB NOT NULL,
                        tags JSONB,
                        created_by VARCHAR(100) NOT NULL,
                        updated_by VARCHAR(100) NOT NULL,
                        created_at TIMESTAMP DEFAULT NOW(),
                        updated_at TIMESTAMP DEFAULT NOW()
);

-- Config table
CREATE TABLE config (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1000),
    active BOOLEAN DEFAULT TRUE,
    schema_id INT REFERENCES schema(id) NOT NULL,
    values JSONB NOT NULL,
    tags JSONB,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(name, schema_id)
);


---- create above / drop below ----

DROP TABLE IF EXISTS Vouchers;
DROP TABLE IF EXISTS Employees;
DROP TABLE IF EXISTS config_versions;
DROP TABLE IF EXISTS config;
DROP TABLE IF EXISTS schema_versions;
DROP TABLE IF EXISTS schema;

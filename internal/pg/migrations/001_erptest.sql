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
                        active BOOLEAN DEFAULT TRUE,
                        active_version_id INT,
                        description VARCHAR(1000),
                        tags JSONB,
                        created_by VARCHAR(100),
                        updated_by VARCHAR(100),
                        created_at TIMESTAMP DEFAULT NOW(),
                        updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE schema_versions (
                                 id SERIAL PRIMARY KEY,
                                 schema_id INT REFERENCES schema(id) ON DELETE CASCADE,
                                 version VARCHAR(50) NOT NULL,
                                 details JSONB NOT NULL,
                                 created_by VARCHAR(255) NOT NULL,
                                 updated_by VARCHAR(255) NOT NULL,
                                 created_at TIMESTAMP DEFAULT NOW(),
                                 updated_at TIMESTAMP DEFAULT NOW(),
                                 UNIQUE(schema_id, version)
);




-- Config table
CREATE TABLE config (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    active_version_id INT,
    description VARCHAR(1000),
    tags JSONB,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE config_versions (
                                 id SERIAL PRIMARY KEY,
                                 config_id INT REFERENCES config(id) ON DELETE CASCADE,
                                 version VARCHAR(50) NOT NULL,
                                 schema_version_id INT REFERENCES schema_versions(id),
                                 values JSONB NOT NULL,
                                 created_by VARCHAR(255),
                                 updated_by VARCHAR(255),
                                 created_at TIMESTAMP DEFAULT NOW(),
                                 updated_at TIMESTAMP DEFAULT NOW(),
                                 UNIQUE(config_id, version)
);



---- create above / drop below ----

DROP TABLE IF EXISTS Vouchers;
DROP TABLE IF EXISTS Employees;
DROP TABLE IF EXISTS config_versions;
DROP TABLE IF EXISTS config;
DROP TABLE IF EXISTS schema_versions;
DROP TABLE IF EXISTS schema;

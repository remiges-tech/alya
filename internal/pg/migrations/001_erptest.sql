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

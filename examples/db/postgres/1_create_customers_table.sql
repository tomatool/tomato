create table "tblCustomers" (
    customer_id serial primary key,
    country char(2) NOT NULL,
    name varchar(255) NOT NULL,
    created_at timestamp default current_timestamp
);

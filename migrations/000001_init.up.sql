create table post (
    id serial primary key,
    link text not null unique,
    created_at timestamptz not null default current_timestamp,
    peer_id int not null
);

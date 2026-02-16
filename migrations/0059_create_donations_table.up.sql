-- Create donations table for tracking user donations to communities
create table if not exists donations (
	id binary(12) not null,
	user_id binary(12) not null,
	community_id binary(12) not null,
	amount int not null default 0,  -- Amount in cents
	currency varchar(3) not null default 'USD',
	status varchar(20) not null default 'pending',  -- pending, completed, failed, refunded
	payment_method varchar(50),  -- stripe, paypal, crypto, etc.
	payment_reference varchar(255),  -- external payment ID/reference
	message text,  -- optional message from donor
	anonymous boolean not null default false,  -- whether to show donor publicly
	created_at datetime not null default current_timestamp(),
	updated_at datetime not null default current_timestamp() on update current_timestamp(),

	primary key (id),
	foreign key (user_id) references users(id) on delete cascade,
	foreign key (community_id) references communities(id) on delete cascade,
	index idx_user_donations (user_id, created_at),
	index idx_community_donations (community_id, created_at),
	index idx_status (status)
);

-- Create community_donation_stats table for aggregated donation statistics
create table if not exists community_donation_stats (
	community_id binary(12) not null,
	total_amount int not null default 0,  -- Total amount raised in cents
	donor_count int not null default 0,  -- Number of unique donors
	donation_count int not null default 0,  -- Total number of donations
	last_donation_at datetime,
	updated_at datetime not null default current_timestamp() on update current_timestamp(),

	primary key (community_id),
	foreign key (community_id) references communities(id) on delete cascade
);

-- Create donation_supporters table for tracking top supporters
create table if not exists donation_supporters (
	id int not null auto_increment,
	community_id binary(12) not null,
	user_id binary(12) not null,
	total_donated int not null default 0,  -- Total donated by this user in cents
	donation_count int not null default 0,
	first_donation_at datetime not null,
	last_donation_at datetime not null,

	primary key (id),
	foreign key (community_id) references communities(id) on delete cascade,
	foreign key (user_id) references users(id) on delete cascade,
	unique key unique_supporter (community_id, user_id),
	index idx_top_supporters (community_id, total_donated desc)
);

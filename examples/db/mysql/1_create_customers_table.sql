CREATE TABLE `customers` (
  `customer_id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `country` char(2) NOT NULL,
  `name` varchar(255) NOT NULL,
  `active` bit(1) DEFAULT b'1',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`customer_id`)
);

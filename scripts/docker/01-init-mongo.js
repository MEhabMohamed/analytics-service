// Ensures the quickbite_analytics database exists on first MongoDB startup.
db = db.getSiblingDB("quickbite_analytics");
db.createCollection("event_ids");


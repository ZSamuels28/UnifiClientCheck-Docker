<?php
// Database.php

class Database {
    private $dbPath;
    public $pdo;

    public function __construct($dbPath) {
        $this->dbPath = $dbPath;
        $this->connect();
    }

    private function connect() {
        $this->pdo = new PDO('sqlite:' . $this->dbPath);
        $this->pdo->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
        $this->pdo->exec("CREATE TABLE IF NOT EXISTS known_macs (id INTEGER PRIMARY KEY, mac_address TEXT UNIQUE)");
    }

    public function loadKnownMacs($envKnownMacs) {
        $knownMacs = array_flip($envKnownMacs); // Use MAC addresses as keys for easy lookup
        $result = $this->pdo->query("SELECT mac_address FROM known_macs");
        foreach ($result as $row) {
            $knownMacs[$row['mac_address']] = true; // Add or overwrite key
        }
        return array_keys($knownMacs); // Convert back to list
    }

    public function updateKnownMacs($mac) {
        $stmt = $this->pdo->prepare("INSERT OR IGNORE INTO known_macs (mac_address) VALUES (?)");
        $stmt->execute([$mac]);
    }

    public function removeOldMacs($clients) {
        $result = $this->pdo->query("SELECT mac_address FROM known_macs");
        foreach ($result as $row) {
            $obj = array_column($clients, null, 'mac')[$row['mac_address']] ?? false;
            if (!$obj){
                echo "Removing device from database: " . ($row['name'] ?? "Unknown") . ", " . ($row['hostname'] ?? "N/A") . ", " . $row['mac_address'] . "\n";
                $stmt = $this->pdo->prepare("DELETE FROM known_macs WHERE mac_address = :mac");
                $stmt->execute(['mac' => $row['mac_address']]);
            }
        }
    }
}
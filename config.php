<?php
/**
 * UniFi Controller configuration
 */

// Controller user details
$controlleruser     = getenv('UNIFI_CONTROLLER_USER') ?: ''; // the user name for access to the UniFi Controller
$controllerpassword = getenv('UNIFI_CONTROLLER_PASSWORD') ?: ''; // the password for access to the UniFi Controller
$controllerurl      = getenv('UNIFI_CONTROLLER_URL') ?: ''; // full URL to the UniFi Controller, eg. 'https://22.22.11.11:8443'
$controllerversion  = getenv('UNIFI_CONTROLLER_VERSION') ?: ''; // the version of the Controller software

// Site ID
$site_id            = getenv('UNIFI_SITE_ID') ?: 'default'; // the site ID

// Debug mode
$debug = false; // set to true to enable debug output to the browser and the PHP error log
?>
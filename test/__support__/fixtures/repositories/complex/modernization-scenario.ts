import { TestRepositoryConfig } from '../../types.js';
import path from 'path';

export const modernizationScenarioConfig: TestRepositoryConfig = {
  repository: {
    name: 'modernization-scenario',
    type: 'legacy-modernization',
    path: path.join(process.cwd(), 'test/fixtures/repositories/complex/modernization-scenario'),
    language: 'multi-language',
    framework: 'legacy-mixed',
    complexity: 'complex',
    description: 'Legacy application requiring modernization and containerization',
    expectedFeatures: [
      'legacy-php-frontend',
      'legacy-java-backend',
      'legacy-perl-scripts',
      'legacy-database-schema',
      'modernization-strategy',
      'incremental-migration'
    ]
  },
  expectation: {
    analysis: {
      language: 'multi-language',
      buildTool: 'mixed',
      packageManager: 'mixed',
      entryPoints: [
        'web/index.php',
        'backend/src/main/java/LegacyApp.java',
        'scripts/process.pl'
      ],
      dependencies: [
        'php:7.4',
        'apache2',
        'mysql',
        'openjdk:8',
        'perl'
      ],
      ports: [80, 8080, 3306],
      environment: {
        APACHE_DOCUMENT_ROOT: '/var/www/html',
        JAVA_OPTS: '-Xmx512m',
        MYSQL_ROOT_PASSWORD: 'legacy_password',
        LEGACY_MODE: 'true'
      }
    },
    dockerfile: {
      baseImage: 'multi-stage',
      workdir: '/app',
      exposedPorts: [80, 8080],
      hasMultiStage: true,
      hasHealthCheck: false, // Legacy apps often lack health checks
      hasNonRootUser: false // Legacy apps often run as root
    },
    k8sManifests: {
      hasDeployment: true,
      hasService: true,
      hasConfigMap: true,
      hasSecret: false,
      hasIngress: true,
      replicas: 1 // Legacy apps often can't scale horizontally
    },
    buildShouldSucceed: true,
    estimatedBuildTimeMs: 240000 // 4 minutes - legacy builds are slow
  }
};

export const modernizationScenarioStructure = {
  // Legacy PHP frontend
  'web/index.php': `<?php
// Legacy PHP application - circa 2008
session_start();

// Security vulnerability: No input validation
$user_input = $_GET['search'] ?? '';

// Legacy database connection - no connection pooling
$db_host = 'localhost';
$db_user = 'root';
$db_pass = 'legacy_password'; // Hardcoded password
$db_name = 'legacy_db';

// Security vulnerability: Direct database connection
$connection = mysql_connect($db_host, $db_user, $db_pass); // Deprecated mysql_* functions
mysql_select_db($db_name, $connection);

// Security vulnerability: SQL injection
$query = "SELECT * FROM users WHERE name LIKE '%$user_input%'";
$result = mysql_query($query);

?>
<!DOCTYPE html>
<html>
<head>
    <title>Legacy Application</title>
    <!-- Legacy inline styles and scripts -->
    <style>
        body { font-family: Arial; background: #f0f0f0; }
        .container { width: 800px; margin: 0 auto; }
        .search-box { padding: 10px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Legacy User Search</h1>
        
        <!-- Security vulnerability: No CSRF protection -->
        <form method="GET">
            <input type="text" name="search" value="<?php echo $user_input; ?>" class="search-box">
            <input type="submit" value="Search">
        </form>

        <?php if ($user_input): ?>
            <h2>Search Results:</h2>
            <?php while($row = mysql_fetch_array($result)): ?>
                <div>
                    <!-- Security vulnerability: XSS -->
                    <p>Name: <?php echo $row['name']; ?></p>
                    <p>Email: <?php echo $row['email']; ?></p>
                </div>
            <?php endwhile; ?>
        <?php endif; ?>

        <!-- Legacy debugging - information disclosure -->
        <?php if ($_GET['debug']): ?>
            <pre>
                <?php
                echo "PHP Version: " . phpversion() . "\\n";
                echo "Database Info: $db_host:$db_user@$db_name\\n";
                print_r($_SERVER);
                ?>
            </pre>
        <?php endif; ?>
    </div>

    <!-- Legacy tracking code -->
    <script>
        // No modern JavaScript practices
        function trackUser() {
            // Security vulnerability: Data sent to insecure endpoint
            var xhr = new XMLHttpRequest();
            xhr.open('GET', 'http://legacy-analytics.com/track?user=' + document.cookie);
            xhr.send();
        }
        trackUser();
    </script>
</body>
</html>
`,

  'web/.htaccess': `# Legacy Apache configuration
RewriteEngine On
RewriteCond %{REQUEST_FILENAME} !-f
RewriteRule ^(.*)$ index.php [L,QSA]

# Security vulnerability: Permissive access
<Files "*.php">
    Order allow,deny
    Allow from all
</Files>

# Legacy PHP settings
php_value display_errors 1
php_value error_reporting E_ALL
php_value memory_limit 256M
php_value upload_max_filesize 50M
`,

  'backend/pom.xml': `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.legacy</groupId>
    <artifactId>legacy-backend</artifactId>
    <version>1.0-SNAPSHOT</version>
    <packaging>jar</packaging>

    <properties>
        <maven.compiler.source>8</maven.compiler.source>
        <maven.compiler.target>8</maven.compiler.target>
        <!-- Using very old versions -->
        <spring.version>4.3.30.RELEASE</spring.version>
        <hibernate.version>4.3.11.Final</hibernate.version>
        <mysql.version>5.1.49</mysql.version>
    </properties>

    <dependencies>
        <dependency>
            <groupId>org.springframework</groupId>
            <artifactId>spring-context</artifactId>
            <version>\${spring.version}</version>
        </dependency>
        
        <dependency>
            <groupId>org.springframework</groupId>
            <artifactId>spring-webmvc</artifactId>
            <version>\${spring.version}</version>
        </dependency>

        <dependency>
            <groupId>org.hibernate</groupId>
            <artifactId>hibernate-core</artifactId>
            <version>\${hibernate.version}</version>
        </dependency>

        <!-- Security vulnerability: Old MySQL driver -->
        <dependency>
            <groupId>mysql</groupId>
            <artifactId>mysql-connector-java</artifactId>
            <version>\${mysql.version}</version>
        </dependency>

        <!-- Legacy logging -->
        <dependency>
            <groupId>log4j</groupId>
            <artifactId>log4j</artifactId>
            <version>1.2.17</version> <!-- Vulnerable version -->
        </dependency>

        <!-- Legacy XML processing -->
        <dependency>
            <groupId>commons-collections</groupId>
            <artifactId>commons-collections</artifactId>
            <version>3.2.1</version> <!-- Vulnerable version -->
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>3.1</version>
                <configuration>
                    <source>8</source>
                    <target>8</target>
                </configuration>
            </plugin>
        </plugins>
    </build>
</project>
`,

  'backend/src/main/java/com/legacy/LegacyApp.java': `package com.legacy;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.ComponentScan;

import java.sql.*;
import java.io.*;
import javax.servlet.http.*;
import java.util.*;

// Legacy Spring Boot application
@SpringBootApplication
@ComponentScan(basePackages = "com.legacy")
public class LegacyApp {
    
    // Security vulnerability: Hardcoded credentials
    private static final String DB_URL = "jdbc:mysql://localhost:3306/legacy_db";
    private static final String DB_USER = "root";
    private static final String DB_PASS = "legacy_password";
    
    public static void main(String[] args) {
        System.setProperty("spring.devtools.restart.enabled", "false");
        SpringApplication.run(LegacyApp.class, args);
    }
    
    // Legacy servlet-style controller
    @RestController
    public class LegacyController {
        
        @RequestMapping("/api/users")
        public String getUsers(HttpServletRequest request) {
            String searchTerm = request.getParameter("search");
            
            try {
                // Security vulnerability: Direct JDBC without connection pooling
                Connection conn = DriverManager.getConnection(DB_URL, DB_USER, DB_PASS);
                
                // Security vulnerability: SQL injection
                String sql = "SELECT * FROM users WHERE name LIKE '%" + searchTerm + "%'";
                Statement stmt = conn.createStatement();
                ResultSet rs = stmt.executeQuery(sql);
                
                StringBuilder result = new StringBuilder();
                result.append("<users>");
                
                while (rs.next()) {
                    result.append("<user>");
                    result.append("<id>").append(rs.getInt("id")).append("</id>");
                    result.append("<name>").append(rs.getString("name")).append("</name>");
                    result.append("<email>").append(rs.getString("email")).append("</email>");
                    result.append("</user>");
                }
                
                result.append("</users>");
                
                // Don't close connections - resource leak
                return result.toString();
                
            } catch (Exception e) {
                // Security vulnerability: Stack trace exposure
                e.printStackTrace();
                return "<error>" + e.getMessage() + "</error>";
            }
        }
        
        @RequestMapping("/api/upload")
        public String uploadFile(HttpServletRequest request) {
            // Security vulnerability: Unrestricted file upload
            try {
                String uploadDir = "/tmp/uploads/";
                String filename = request.getParameter("filename");
                
                // Security vulnerability: Path traversal
                File uploadFile = new File(uploadDir + filename);
                
                // Legacy file handling - no validation
                FileOutputStream fos = new FileOutputStream(uploadFile);
                // ... file writing code ...
                
                return "File uploaded successfully";
                
            } catch (Exception e) {
                return "Upload failed: " + e.getMessage();
            }
        }
    }
}
`,

  'scripts/process.pl': `#!/usr/bin/perl
# Legacy Perl processing script - circa 2005

use strict;
use warnings;
use DBI;
use CGI;

# Security vulnerability: Hardcoded credentials
my $dsn = "DBI:mysql:database=legacy_db;host=localhost";
my $username = "root";
my $password = "legacy_password";

# Legacy CGI processing
my $cgi = CGI->new;
my $action = $cgi->param('action') || '';

print $cgi->header('text/html');

if ($action eq 'process') {
    process_data();
} elsif ($action eq 'cleanup') {
    cleanup_data();
} else {
    show_form();
}

sub process_data {
    my $user_input = $cgi->param('data') || '';
    
    # Security vulnerability: No input validation
    my $dbh = DBI->connect($dsn, $username, $password) 
        or die "Cannot connect: $DBI::errstr";
    
    # Security vulnerability: SQL injection
    my $sql = "INSERT INTO processed_data (data, timestamp) VALUES ('$user_input', NOW())";
    $dbh->do($sql);
    
    print "<h1>Data Processed Successfully</h1>";
    print "<p>Processed: $user_input</p>";
    
    # Don't close database handle - resource leak
}

sub cleanup_data {
    # Security vulnerability: No authentication
    my $dbh = DBI->connect($dsn, $username, $password);
    
    # Dangerous operation without confirmation
    $dbh->do("DELETE FROM processed_data WHERE timestamp < DATE_SUB(NOW(), INTERVAL 30 DAY)");
    
    print "<h1>Cleanup Complete</h1>";
}

sub show_form {
    print <<HTML;
<html>
<head><title>Legacy Perl Processor</title></head>
<body>
    <h1>Data Processor</h1>
    <form method="POST">
        <input type="hidden" name="action" value="process">
        <textarea name="data" rows="10" cols="50"></textarea><br>
        <input type="submit" value="Process Data">
    </form>
    
    <hr>
    
    <form method="POST">
        <input type="hidden" name="action" value="cleanup">
        <input type="submit" value="Cleanup Old Data" onclick="return confirm('Are you sure?')">
    </form>
</body>
</html>
HTML
}

1;
`,

  'config/legacy.conf': `# Legacy configuration file
[database]
host=localhost
port=3306
username=root
password=legacy_password  # Security vulnerability: Plain text password
database=legacy_db

[application]
debug=true  # Security vulnerability: Debug enabled in production
log_level=DEBUG
error_display=true
max_memory=512M
timeout=300

[security]
# Security vulnerability: Weak configuration
ssl_enabled=false
csrf_protection=false
session_security=low
password_complexity=false
encryption_key=simple123  # Weak encryption key

[paths]
upload_path=/tmp/uploads
log_path=/var/log/legacy
temp_path=/tmp/legacy

[features]
# Legacy feature flags
enable_legacy_auth=true
allow_file_upload=true
enable_debug_endpoints=true
bypass_security_checks=true  # Security vulnerability
`,

  'docker-compose.legacy.yml': `version: '3.8'
services:
  legacy-web:
    build:
      context: .
      dockerfile: Dockerfile.php
    ports:
      - "80:80"
    volumes:
      - ./web:/var/www/html
    environment:
      - PHP_DISPLAY_ERRORS=1
      - MYSQL_HOST=legacy-db
    depends_on:
      - legacy-db
    # Security vulnerability: No resource limits or security context

  legacy-backend:
    build:
      context: .
      dockerfile: Dockerfile.java
    ports:
      - "8080:8080"
    environment:
      - SPRING_PROFILES_ACTIVE=legacy
      - DB_URL=jdbc:mysql://legacy-db:3306/legacy_db
    depends_on:
      - legacy-db
    # Security vulnerability: No health checks

  legacy-db:
    image: mysql:5.7  # Old MySQL version
    environment:
      - MYSQL_ROOT_PASSWORD=legacy_password  # Weak password
      - MYSQL_DATABASE=legacy_db
    ports:
      - "3306:3306"  # Security vulnerability: Exposed database port
    volumes:
      - ./database/legacy_schema.sql:/docker-entrypoint-initdb.d/schema.sql
    # Security vulnerability: No backup strategy

  legacy-scripts:
    build:
      context: .
      dockerfile: Dockerfile.perl
    volumes:
      - ./scripts:/opt/scripts
      - ./data:/opt/data
    environment:
      - PERL_ENV=production
    # Security vulnerability: Root access to host filesystem
`,

  'Dockerfile.php': `# Legacy PHP Dockerfile
FROM php:7.4-apache

# Security vulnerability: Running as root
WORKDIR /var/www/html

# Install legacy PHP extensions
RUN docker-php-ext-install mysql pdo pdo_mysql

# Security vulnerability: Copy all files including sensitive ones
COPY web/ /var/www/html/
COPY config/ /etc/legacy/

# Security vulnerability: Permissive file permissions
RUN chmod -R 777 /var/www/html

# Enable legacy PHP settings
RUN echo "display_errors = On" >> /usr/local/etc/php/php.ini
RUN echo "error_reporting = E_ALL" >> /usr/local/etc/php/php.ini
RUN echo "log_errors = On" >> /usr/local/etc/php/php.ini

EXPOSE 80

# Security vulnerability: No health check
CMD ["apache2-foreground"]
`,

  'database/legacy_schema.sql': `-- Legacy database schema
CREATE DATABASE IF NOT EXISTS legacy_db;
USE legacy_db;

-- Users table with weak security
CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL,  -- Plain text passwords
    role ENUM('user', 'admin') DEFAULT 'user',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert test data with weak passwords
INSERT INTO users (name, email, password, role) VALUES 
('admin', 'admin@legacy.com', 'admin123', 'admin'),  -- Weak password
('john', 'john@legacy.com', 'password', 'user'),     -- Weak password
('jane', 'jane@legacy.com', '123456', 'user');       -- Weak password

-- Sessions table for legacy session management
CREATE TABLE user_sessions (
    session_id VARCHAR(255) PRIMARY KEY,
    user_id INT,
    data TEXT,
    expires DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Processed data table
CREATE TABLE processed_data (
    id INT AUTO_INCREMENT PRIMARY KEY,
    data TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status ENUM('pending', 'processed', 'failed') DEFAULT 'pending'
);

-- Legacy audit log - minimal security
CREATE TABLE audit_log (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    action VARCHAR(255),
    details TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Grant overly permissive privileges
GRANT ALL PRIVILEGES ON legacy_db.* TO 'root'@'%' IDENTIFIED BY 'legacy_password';
FLUSH PRIVILEGES;
`
};
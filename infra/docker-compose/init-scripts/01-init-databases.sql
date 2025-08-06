-- Initialize Nexus Platform Databases
-- This script creates the necessary databases and schemas for the Nexus platform

-- Create databases for different services
CREATE DATABASE nexus_auth;
CREATE DATABASE nexus_users;
CREATE DATABASE nexus_config;
CREATE DATABASE nexus_analytics;
CREATE DATABASE nexus_audit;

-- Create users for different services
CREATE USER auth_service WITH PASSWORD 'auth_password';
CREATE USER user_service WITH PASSWORD 'user_password';
CREATE USER config_service WITH PASSWORD 'config_password';
CREATE USER analytics_service WITH PASSWORD 'analytics_password';
CREATE USER audit_service WITH PASSWORD 'audit_password';

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE nexus_auth TO auth_service;
GRANT ALL PRIVILEGES ON DATABASE nexus_users TO user_service;
GRANT ALL PRIVILEGES ON DATABASE nexus_config TO config_service;
GRANT ALL PRIVILEGES ON DATABASE nexus_analytics TO analytics_service;
GRANT ALL PRIVILEGES ON DATABASE nexus_audit TO audit_service;

-- Connect to nexus_auth database and create initial schema
\c nexus_auth;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role VARCHAR(50) NOT NULL,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role, permission_id)
);

-- Insert default permissions
INSERT INTO permissions (name, description) VALUES
    ('read:users', 'Read user information'),
    ('write:users', 'Create and update users'),
    ('delete:users', 'Delete users'),
    ('read:config', 'Read configuration'),
    ('write:config', 'Update configuration'),
    ('read:analytics', 'Read analytics data'),
    ('admin:all', 'Full administrative access');

-- Connect to nexus_config database and create schema
\c nexus_config;

CREATE TABLE IF NOT EXISTS configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(255) UNIQUE NOT NULL,
    value JSONB NOT NULL,
    environment VARCHAR(50) NOT NULL DEFAULT 'development',
    version INTEGER NOT NULL DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS feature_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    is_enabled BOOLEAN DEFAULT false,
    conditions JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default configurations
INSERT INTO configurations (key, value, environment) VALUES
    ('api.rate_limit', '{"requests_per_minute": 1000}', 'development'),
    ('auth.session_timeout', '{"minutes": 60}', 'development'),
    ('logging.level', '{"level": "debug"}', 'development'),
    ('monitoring.enabled', '{"enabled": true}', 'development');

-- Connect to nexus_audit database and create schema
\c nexus_audit;

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    service_name VARCHAR(100) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_service_name ON audit_logs(service_name);
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);

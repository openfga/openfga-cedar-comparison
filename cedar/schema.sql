-- Cedar Authorization Example - Database Schema and Test Data
-- This script creates all the tables and data needed for the blog post example

-- Drop tables if they exist (for clean setup)
DROP TABLE IF EXISTS folder_permissions;
DROP TABLE IF EXISTS document_permissions;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;

-- Create Organizations table
CREATE TABLE organizations (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL
);

-- Create Users table
CREATE TABLE users (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100)
);

-- Create Folders table
CREATE TABLE folders (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    organization_id VARCHAR(50) NOT NULL REFERENCES organizations(id),
    owner_id VARCHAR(50) REFERENCES users(id)
);

-- Create Documents table
CREATE TABLE documents (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    organization_id VARCHAR(50) NOT NULL REFERENCES organizations(id),
    owner_id VARCHAR(50) REFERENCES users(id),
    folder_id VARCHAR(50) REFERENCES folders(id)
);

-- Create Organization Members table (many-to-many relationship)
CREATE TABLE organization_members (
    user_id VARCHAR(50) NOT NULL REFERENCES users(id),
    organization_id VARCHAR(50) NOT NULL REFERENCES organizations(id),
    PRIMARY KEY (user_id, organization_id)
);

-- Create Document Permissions table
CREATE TABLE document_permissions (
    id SERIAL PRIMARY KEY,
    document_id VARCHAR(50) NOT NULL REFERENCES documents(id),
    user_id VARCHAR(50) NOT NULL REFERENCES users(id),
    permission_type VARCHAR(20) NOT NULL CHECK (permission_type IN ('viewer', 'editor')),
    UNIQUE(document_id, user_id, permission_type)
);

-- Create Folder Permissions table
CREATE TABLE folder_permissions (
    id SERIAL PRIMARY KEY,
    folder_id VARCHAR(50) NOT NULL REFERENCES folders(id),
    user_id VARCHAR(50) NOT NULL REFERENCES users(id),
    permission_type VARCHAR(20) NOT NULL CHECK (permission_type IN ('viewer', 'editor')),
    UNIQUE(folder_id, user_id, permission_type)
);

-- Insert test data
-- Organizations
INSERT INTO organizations (id, name) VALUES 
    ('org1', 'Tech Corp'),
    ('org2', 'Marketing Inc');

-- Users
INSERT INTO users (id, name, email) VALUES 
    ('alice', 'Alice Johnson', 'alice@techcorp.com'),
    ('bob', 'Bob Smith', 'bob@techcorp.com'),
    ('charlie', 'Charlie Brown', 'charlie@techcorp.com'),
    ('david', 'David Wilson', 'david@marketing.com'),
    ('eve', 'Eve Davis', 'eve@marketing.com');

-- Organization memberships
INSERT INTO organization_members (user_id, organization_id) VALUES 
    ('alice', 'org1'),
    ('bob', 'org1'),
    ('charlie', 'org1'),
    ('david', 'org2'),
    ('eve', 'org2');

-- Folders
INSERT INTO folders (id, name, organization_id, owner_id) VALUES 
    ('folder1', 'Engineering Docs', 'org1', 'alice'),
    ('folder2', 'Marketing Materials', 'org2', 'david');

-- Documents
INSERT INTO documents (id, name, organization_id, owner_id, folder_id) VALUES 
    ('doc1', 'Architecture Guide', 'org1', 'alice', 'folder1'),
    ('doc2', 'API Documentation', 'org1', 'bob', 'folder1'),
    ('doc3', 'Marketing Strategy', 'org2', 'david', 'folder2'),
    ('doc4', 'Public Document', 'org1', 'alice', NULL);

-- Document permissions
INSERT INTO document_permissions (document_id, user_id, permission_type) VALUES 
    ('doc2', 'charlie', 'viewer'),
    ('doc4', 'bob', 'editor'),
    ('doc4', 'charlie', 'viewer');

-- Folder permissions (these apply to all documents in the folder)
INSERT INTO folder_permissions (folder_id, user_id, permission_type) VALUES 
    ('folder1', 'bob', 'viewer'),
    ('folder2', 'eve', 'editor');

-- Verify the setup with some sample queries
SELECT 'Setup verification:' as status;

SELECT 'Organizations:' as table_name;
SELECT * FROM organizations;

SELECT 'Users and their organizations:' as info;
SELECT u.id, u.name, o.name as organization 
FROM users u 
JOIN organization_members om ON u.id = om.user_id 
JOIN organizations o ON om.organization_id = o.id;

SELECT 'Documents and ownership:' as info;
SELECT d.id, d.name, d.organization_id, d.owner_id, d.folder_id 
FROM documents d;

SELECT 'Document permissions:' as info;
SELECT dp.document_id, dp.user_id, dp.permission_type 
FROM document_permissions dp;

SELECT 'Folder permissions:' as info;
SELECT fp.folder_id, fp.user_id, fp.permission_type 
FROM folder_permissions fp;

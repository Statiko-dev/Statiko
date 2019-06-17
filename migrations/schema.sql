CREATE TABLE IF NOT EXISTS "schema_migration" (
"version" TEXT NOT NULL
);
CREATE UNIQUE INDEX "schema_migration_version_idx" ON "schema_migration" (version);
CREATE TABLE IF NOT EXISTS "sites" (
"id" TEXT NOT NULL,
"client_caching" NUMERIC NOT NULL,
"tls_certificate" TEXT NOT NULL,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
CREATE UNIQUE INDEX "sites_id_idx" ON "sites" (id);
CREATE TABLE IF NOT EXISTS "domains" (
"id" TEXT NOT NULL,
"site_id" TEXT NOT NULL,
"domain" TEXT NOT NULL,
"is_default" NUMERIC NOT NULL,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
CREATE UNIQUE INDEX "domains_id_idx" ON "domains" (id);
CREATE INDEX "domains_site_id_idx" ON "domains" (site_id);
CREATE UNIQUE INDEX "domains_domain_idx" ON "domains" (domain);
CREATE TABLE IF NOT EXISTS "deployments" (
"id" TEXT NOT NULL,
"site_id" TEXT NOT NULL,
"app_name" TEXT NOT NULL,
"app_version" TEXT NOT NULL,
"status" INTEGER NOT NULL,
"error" TEXT,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
CREATE UNIQUE INDEX "deployments_id_idx" ON "deployments" (id);
CREATE INDEX "deployments_site_id_status_idx" ON "deployments" (site_id, status);
CREATE INDEX "deployments_app_name_app_version_idx" ON "deployments" (app_name, app_version);
CREATE INDEX "deployments_updated_at_idx" ON "deployments" (updated_at);

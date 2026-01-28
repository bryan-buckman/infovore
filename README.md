On startup, app reads DB_URL from environment variables or /data/.env
Users can configure the database URL via Settings in the web UI
Changes are saved to the .env file
App restart is required to pick up the new database connection
For Kubernetes: mount a Secret containing .env to /data/.env
